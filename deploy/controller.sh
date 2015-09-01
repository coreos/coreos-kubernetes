#!/bin/bash -e

K8S_VER="v1.0.3"

function usage {
    echo "USAGE: $0 <command>"
    echo "Commands:"
    echo -e "\tinit \tInitialize control node services"
    echo -e "\tstart \tStart control node services"
}

if [ -z $1 ]; then
    usage
    exit 1
fi

CMD=$1

function init_config {
    local REQUIRED=('ADVERTISE_IP' 'POD_NETWORK' 'ETCD_ENDPOINTS' 'SERVICE_IP_RANGE' 'K8S_SERVICE_IP' 'DNS_SERVICE_IP' )

    if [ -z $ADVERTISE_IP ]; then
        export ADVERTISE_IP=$(awk -F= '/COREOS_PUBLIC_IPV4/ {print $2}' /etc/environment)
    fi
    if [ -z $POD_NETWORK ]; then
        export POD_NETWORK="10.2.0.0/16"
    fi
    if [ -z $ETCD_ENDPOINTS ]; then
        export ETCD_ENDPOINTS="http://127.0.0.1:2379"
    fi
    if [ -z $SERVICE_IP_RANGE ]; then
        export SERVICE_IP_RANGE="10.3.0.0/24"
    fi
    if [ -z $K8S_SERVICE_IP ]; then
        export K8S_SERVICE_IP="10.3.0.1"
    fi
    if [ -z $DNS_SERVICE_IP ]; then
        export DNS_SERVICE_IP="10.3.0.10"
    fi
    for REQ in "${REQUIRED[@]}"; do
        if [ -z "$(eval echo \$$REQ)" ]; then
            echo "Missing required config value: ${REQ}"
            exit 1
        fi
    done

    # For use in single-node deployments
    if [ "$REGISTER_NODE" != "true" ]; then
        export REGISTER_NODE="false"
    fi
}

function init_flannel {
    echo "Waiting for etcd..."
    while true
    do
        IFS=',' read -ra ES <<< "$ETCD_ENDPOINTS"
        for ETCD in "${ES[@]}"; do
            echo "Trying: $ETCD"
            if [ -n "$(curl --silent "$ETCD/v2/machines")" ]; then
                local ACTIVE_ETCD=$ETCD
                break
            fi
            sleep 1
        done
        if [ -n "$ACTIVE_ETCD" ]; then
            break
        fi
    done
    RES=$(curl --silent -X PUT -d "value={\"Network\":\"$POD_NETWORK\"}" "$ACTIVE_ETCD/v2/keys/coreos.com/network/config?prevExist=false")
    if [ -z "$(echo $RES | grep '"action":"create"')" ] && [ -z "$(echo $RES | grep 'Key already exists')" ]; then
        echo "Unexpected error configuring flannel pod network: $RES"
    fi
}

function init_docker {
    local TEMPLATE=/etc/systemd/system/docker.service.d/40-flannel.conf
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Unit]
Requires=flanneld.service
After=flanneld.service
EOF
    }

    # reload now before docker commands are run in later
    # init steps or dockerd will start before flanneld
    systemctl daemon-reload
}

function init_kubernetes_release {
    local RELEASE_DIR=/opt/kubernetes_release/$K8S_VER
    mkdir -p $RELEASE_DIR

    [ -f $RELEASE_DIR/kubernetes-server-linux-amd64.tar.gz ] || {
        echo "K8S: downloading release: $K8S_VER"
        curl --silent -o $RELEASE_DIR/kubernetes-server-linux-amd64.tar.gz  https://storage.googleapis.com/kubernetes-release/release/$K8S_VER/kubernetes-server-linux-amd64.tar.gz
    }

    [ -d $RELEASE_DIR/kubernetes/server ] || {
        echo "K8S: extracting release: $K8S_VER"
        tar xzf $RELEASE_DIR/kubernetes-server-linux-amd64.tar.gz -C $RELEASE_DIR
    }

    mkdir -p /opt/bin
    BINS=( "kubectl" "kubelet" "kube-proxy" )
    for BIN in "${BINS[@]}"; do
      [ -x /opt/bin/$BIN ] || {
        echo "K8S BIN: $BIN"
        cp $RELEASE_DIR/kubernetes/server/bin/$BIN /opt/bin/$BIN
        chown core:core /opt/bin/$BIN
      }
    done

    local REPO="gcr.io/google_containers"
    local IMAGES=( "kube-apiserver" "kube-scheduler" "kube-controller-manager" )
    for IMG in "${IMAGES[@]}"; do
        local IMG_TAG=$(cat $RELEASE_DIR/kubernetes/server/bin/$IMG.docker_tag)
        if [ "$IMG_TAG" != "$(docker images $REPO/$IMG | awk '/$IMG/ {print $2}')" ]; then
            echo "K8S IMAGE: $REPO/$IMG:$IMG_TAG"
            docker load -i $RELEASE_DIR/kubernetes/server/bin/$IMG.tar
        fi
        # export image name variables for use in templates. e.g. KUBE_APISERVER_IMAGE
        local NAME=$(echo $IMG | awk '{print toupper($0)}' | tr '-' '_')
        export $(echo "$NAME"_IMAGE=$REPO/$IMG:$IMG_TAG)
    done
}

function init_templates {
    local TEMPLATE=/etc/systemd/system/kubelet.service
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
ExecStart=/opt/bin/kubelet \
  --api_servers=http://127.0.0.1:8080 \
  --register-node=${REGISTER_NODE} \
  --config=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster_dns=${DNS_SERVICE_IP} \
  --cluster_domain=cluster.local \
  --cadvisor-port=0
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    }

    local TEMPLATE=/etc/systemd/system/kube-proxy.service
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
ExecStart=/opt/bin/kube-proxy --master=http://127.0.0.1:8080 --logtostderr
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    }

    local TEMPLATE=/etc/kubernetes/manifests/kube-apiserver.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-apiserver
    image: $KUBE_APISERVER_IMAGE
    command:
    - /bin/sh
    - -c
    - /usr/local/bin/kube-apiserver --bind-address=0.0.0.0 --etcd_servers=${ETCD_ENDPOINTS} --service-cluster-ip-range=${SERVICE_IP_RANGE} --secure_port=443 --advertise-address=${ADVERTISE_IP}
    ports:
    - containerPort: 443
      hostPort: 443
      name: https
    - containerPort: 7080
      hostPort: 7080
      name: http
    - containerPort: 8080
      hostPort: 8080
      name: local
    volumeMounts:
    - mountPath: /etc/ssl
      name: etcssl
      readOnly: true
    - mountPath: /var/ssl
      name: varssl
      readOnly: true
    - mountPath: /etc/openssl
      name: etcopenssl
      readOnly: true
    - mountPath: /etc/pki/tls
      name: etcpkitls
      readOnly: true
  volumes:
  - hostPath:
      path: /etc/ssl
    name: etcssl
  - hostPath:
      path: /var/ssl
    name: varssl
  - hostPath:
      path: /etc/openssl
    name: etcopenssl
  - hostPath:
      path: /etc/pki/tls
    name: etcpkitls
EOF
    }

    local TEMPLATE=/etc/kubernetes/manifests/kube-podmaster.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-podmaster
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: scheduler-elector
    image: gcr.io/google_containers/podmaster:1.1
    command:
    - /podmaster
    - --etcd-servers=${ETCD_ENDPOINTS}
    - --key=scheduler
    - --whoami=${ADVERTISE_IP}
    - --source-file=/src/manifests/kube-scheduler.yaml
    - --dest-file=/dst/manifests/kube-scheduler.yaml
    volumeMounts:
    - mountPath: /src/manifests
      name: manifest-src
      readOnly: true
    - mountPath: /dst/manifests
      name: manifest-dst
  - name: controller-manager-elector
    image: gcr.io/google_containers/podmaster:1.1
    command:
    - /podmaster
    - --etcd-servers=${ETCD_ENDPOINTS}
    - --key=controller
    - --whoami=${ADVERTISE_IP}
    - --source-file=/src/manifests/kube-controller-manager.yaml
    - --dest-file=/dst/manifests/kube-controller-manager.yaml
    terminationMessagePath: /dev/termination-log
    volumeMounts:
    - mountPath: /src/manifests
      name: manifest-src
      readOnly: true
    - mountPath: /dst/manifests
      name: manifest-dst
  volumes:
  - hostPath:
      path: /srv/kubernetes/manifests
    name: manifest-src
  - hostPath:
      path: /etc/kubernetes/manifests
    name: manifest-dst
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/kube-controller-manager.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - command:
    - /bin/sh
    - -c
    - /usr/local/bin/kube-controller-manager --master=http://127.0.0.1:8080
    image: $KUBE_CONTROLLER_MANAGER_IMAGE
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10252
      initialDelaySeconds: 15
      timeoutSeconds: 1
    name: kube-controller-manager
    volumeMounts:
    - mountPath: /etc/ssl
      name: etcssl
      readOnly: true
    - mountPath: /var/ssl
      name: varssl
      readOnly: true
    - mountPath: /etc/openssl
      name: etcopenssl
      readOnly: true
    - mountPath: /etc/pki/tls
      name: etcpkitls
      readOnly: true
  hostNetwork: true
  volumes:
  - hostPath:
      path: /etc/ssl
    name: etcssl
  - hostPath:
      path: /var/ssl
    name: varssl
  - hostPath:
      path: /etc/openssl
    name: etcopenssl
  - hostPath:
      path: /etc/pki/tls
    name: etcpkitls
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/kube-scheduler.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-scheduler
    image: $KUBE_SCHEDULER_IMAGE
    command:
    - /bin/sh
    - -c
    - /usr/local/bin/kube-scheduler --master=http://127.0.0.1:8080
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10251
      initialDelaySeconds: 15
      timeoutSeconds: 1
EOF
    }

    local DNS_KUBECONFIG="
apiVersion: v1
kind: Config
clusters:
- name: default
  cluster:
     server: https://$K8S_SERVICE_IP
     insecure-skip-tls-verify: true
contexts:
- context:
    cluster: default
    namespace: kube-system
  name: dns-context
current-context: dns-context"

    local TEMPLATE=/srv/kubernetes/manifests/kube-dns.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Secret
metadata:
  name: dns-kubeconfig
  namespace: kube-system
data:
  kubeconfig: $(echo "$DNS_KUBECONFIG" | base64 --wrap=0)

---

apiVersion: v1
kind: ReplicationController
metadata:
  name: kube-dns-v8
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    version: v8
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 1
  selector:
    k8s-app: kube-dns
    version: v8
  template:
    metadata:
      labels:
        k8s-app: kube-dns
        version: v8
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
      - name: etcd
        image: gcr.io/google_containers/etcd:2.0.9
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        command:
        - /usr/local/bin/etcd
        - -data-dir
        - /var/etcd/data
        - -listen-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -advertise-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -initial-cluster-token
        - skydns-etcd
        volumeMounts:
        - name: etcd-storage
          mountPath: /var/etcd/data
      - name: kube2sky
        image: gcr.io/google_containers/kube2sky:1.11
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        args:
        # command = "/kube2sky"
        - -domain=cluster.local
        - --kubecfg_file=/config/kubeconfig
        volumeMounts:
        - name: sslcerts
          mountPath: /etc/ssl/certs/ca-certificates.crt
        - name: kubeconfig
          mountPath: /config
          readOnly: true
      - name: skydns
        image: gcr.io/google_containers/skydns:2015-03-11-001
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        args:
        # command = "/skydns"
        - -machines=http://localhost:4001
        - -addr=0.0.0.0:53
        - -domain=cluster.local.
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
      - name: healthz
        image: gcr.io/google_containers/exechealthz:1.0
        resources:
          limits:
            cpu: 10m
            memory: 20Mi
        args:
        - -cmd=nslookup kubernetes.default.svc.cluster.local localhost >/dev/null
        - -port=8080
        ports:
        - containerPort: 8080
          protocol: TCP
      volumes:
      - name: etcd-storage
        emptyDir: {}
      - name: sslcerts
        hostPath:
          path: /etc/ssl/certs/ca-certificates.crt
      - name: kubeconfig
        secret:
          secretName: dns-kubeconfig
      dnsPolicy: Default  # Don't use cluster DNS.

---

apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/name: "KubeDNS"
    kubernetes.io/cluster-service: "true"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: $DNS_SERVICE_IP
  ports:
    - protocol: UDP
      name: dns
      port: 53
    - protocol: TCP
      name: dns-tcp
      port: 53
EOF
    }

}

function start_addons {
    echo "Waiting for Kubernetes API..."
    until curl --silent "http://127.0.0.1:8080/version"
    do
        sleep 5
    done
    echo
    echo "K8S: DNS addon"
    /opt/bin/kubectl create -f /srv/kubernetes/manifests/kube-dns.yaml
}

if [ "$CMD" == "init" ]; then
    echo "Starting initialization"
    init_config
    init_flannel
    init_docker
    init_kubernetes_release
    init_templates
    echo "Initialization complete"
    exit 0
fi

if [ "$CMD" == "start" ]; then
    echo "Starting services"
    systemctl daemon-reload
    systemctl enable kubelet; systemctl start kubelet
    systemctl enable kube-proxy; systemctl start kube-proxy
    start_addons
    echo "Service start complete"
    exit 0
fi

usage
exit 1
