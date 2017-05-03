#!/bin/bash
set -e

# IP address of this node
export ADVERTISE_IP=10.0.0.1

# List of etcd servers (http://ip:port), comma separated
export ETCD_ENDPOINTS=https://10.0.0.1:2379,https://10.0.0.2:2379,https://10.0.0.3:2379

# Specify the version (vX.Y.Z) of Kubernetes assets to deploy
export K8S_VER=v1.6.2_coreos.0

# Hyperkube image repository to use.
export HYPERKUBE_IMAGE_REPO=quay.io/coreos/hyperkube

# The CIDR network to use for pod IPs.
# Each pod launched in the cluster will be assigned an IP out of this range.
# Each node will be configured such that these IPs will be routable using the flannel overlay network.
export POD_NETWORK=10.2.0.0/16

# The CIDR network to use for service cluster IPs.
# Each service will be assigned a cluster IP out of this range.
# This must not overlap with any IP ranges assigned to the POD_NETWORK, or other existing network infrastructure.
# Routing to these IPs is handled by a proxy service local to each node, and are not required to be routable between nodes.
export SERVICE_IP_RANGE=10.3.0.0/24

# The IP address of the Kubernetes API Service
# If the SERVICE_IP_RANGE is changed above, this must be set to the first IP in that range.
export K8S_SERVICE_IP=10.3.0.1

# The IP address of the cluster DNS service.
# This IP must be in the range of the SERVICE_IP_RANGE and cannot be the first IP in the range.
# This same IP must be configured on all worker nodes to enable DNS service discovery.
export DNS_SERVICE_IP=10.3.0.10

# Whether to use Calico for Kubernetes network policy.
export USE_CALICO=true

# Determines the container runtime for kubernetes to use. Accepts 'docker' or 'rkt'.
export CONTAINER_RUNTIME=docker

# The above settings can optionally be overridden using an environment file:
ENV_FILE=/run/coreos-kubernetes/options.env

# Folder for systemd services
systemd_svc_dir="/etc/systemd/system"

# Folder for k8s control-plane manifests
k8s_manifests_dir="/etc/kubernetes/manifests"

# Folder for non-k8s manifests
manifests_dir="/srv/kubernetes/manifests"

# To run a self hosted Calico install it needs to be able to write to the CNI dir
if [ "${USE_CALICO}" = "true" ]; then
    export CALICO_OPTS="--volume cni-bin,kind=host,source=/opt/cni/bin \\
  --mount volume=cni-bin,target=/opt/cni/bin"
else
    export CALICO_OPTS=""
fi

# -------------

function init_config {
    local REQUIRED=('ADVERTISE_IP' 'POD_NETWORK' 'ETCD_ENDPOINTS' 'SERVICE_IP_RANGE' 'K8S_SERVICE_IP' 'DNS_SERVICE_IP' 'K8S_VER' 'HYPERKUBE_IMAGE_REPO' 'USE_CALICO')

    if [ -f $ENV_FILE ]; then
        export $(cat $ENV_FILE | xargs)
    fi

    if [ -z $ADVERTISE_IP ]; then
        export ADVERTISE_IP=$(awk -F= '/COREOS_PUBLIC_IPV4/ {print $2}' /etc/environment)
    fi

    for REQ in "${REQUIRED[@]}"; do
        if [ -z "$(eval echo \$$REQ)" ]; then
            echo "Missing required config value: ${REQ}"
            exit 1
        fi
    done
}

function init_flannel {
    echo "Waiting for etcd..."
    while true
    do
        IFS=',' read -ra ES <<< "$ETCD_ENDPOINTS"
        for ETCD in "${ES[@]}"; do
            echo "Trying: $ETCD"
            if [ -n "$(sudo curl --cacert /etc/etcd/ssl/ca.pem --cert /etc/etcd/ssl/peer.pem --key /etc/etcd/ssl/peer-key.pem --silent "$ETCD/v2/machines")" ]; then
                local ACTIVE_ETCD=$ETCD
                break
            fi
            sleep 1
        done
        if [ -n "$ACTIVE_ETCD" ]; then
            break
        fi
    done
    RES=$(sudo curl --cacert /etc/etcd/ssl/ca.pem --cert /etc/etcd/ssl/peer.pem --key /etc/etcd/ssl/peer-key.pem --silent -X PUT -d "value={\"Network\":\"$POD_NETWORK\",\"Backend\":{\"Type\":\"vxlan\"}}" "$ACTIVE_ETCD/v2/keys/coreos.com/network/config?prevExist=false")
    if [ -z "$(echo $RES | grep '"action":"create"')" ] && [ -z "$(echo $RES | grep 'Key already exists')" ]; then
        echo "Unexpected error configuring flannel pod network: $RES"
    fi
}

function init_templates {
    local TEMPLATE=$systemd_svc_dir/kubelet.service
    local uuid_file="/var/run/kubelet-pod.uuid"
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
Environment=KUBELET_IMAGE_TAG=${K8S_VER}
Environment=KUBELET_IMAGE_URL=${HYPERKUBE_IMAGE_REPO}
Environment="RKT_RUN_ARGS=--uuid-file-save=${uuid_file} \\
  --volume calico-tls,kind=host,source=/etc/calico/ssl \\
  --mount volume=calico-tls,target=/etc/calico/ssl \\
  --volume tls,kind=host,source=/etc/kubernetes/ssl/kubelet \\
  --mount volume=tls,target=/etc/kubernetes/ssl/kubelet \\
  --volume kubeconfig,kind=host,source=/etc/kubernetes/kubeconfig/kubelet.yaml \\
  --mount volume=kubeconfig,target=/etc/kubernetes/kubeconfig/kubelet.yaml \\
  --volume dns,kind=host,source=/etc/resolv.conf \\
  --mount volume=dns,target=/etc/resolv.conf \\
  --volume rkt,kind=host,source=/opt/bin/host-rkt \\
  --mount volume=rkt,target=/usr/bin/rkt \\
  --volume var-lib-rkt,kind=host,source=/var/lib/rkt \\
  --mount volume=var-lib-rkt,target=/var/lib/rkt \\
  --volume stage,kind=host,source=/tmp \\
  --mount volume=stage,target=/tmp \\
  --volume var-log,kind=host,source=/var/log \\
  --mount volume=var-log,target=/var/log \\
  ${CALICO_OPTS}"
ExecStartPre=/usr/bin/mkdir -p $k8s_manifests_dir
ExecStartPre=/usr/bin/mkdir -p /opt/cni/bin
ExecStartPre=/usr/bin/mkdir -p /var/log/containers
ExecStartPre=-/usr/bin/rkt rm --uuid-file=${uuid_file}
ExecStart=/usr/lib/coreos/kubelet-wrapper \\
  --client-ca-file=/etc/kubernetes/ssl/kubelet/ca.pem \\
  --tls-cert-file=/etc/kubernetes/ssl/kubelet/server.pem \\
  --tls-private-key-file=/etc/kubernetes/ssl/kubelet/server-key.pem \\
  --kubeconfig=/etc/kubernetes/kubeconfig/kubelet.yaml \\
  --api-servers=https://127.0.0.1:6443 \\
  --register-schedulable=false \\
  --network-plugin=cni \\
  --network-plugin-dir=/etc/kubernetes/cni/net.d \\
  --cni-bin-dir=/opt/cni/bin \\
  --container-runtime=${CONTAINER_RUNTIME} \\
  --rkt-path=/usr/bin/rkt \\
  --rkt-stage1-image=coreos.com/rkt/stage1-coreos \\
  --allow-privileged=true \\
  --pod-manifest-path=$k8s_manifests_dir \\
  --cluster_dns=${DNS_SERVICE_IP} \\
  --cluster_domain=cluster.local
ExecStop=-/usr/bin/rkt stop --uuid-file=${uuid_file}
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    fi

    local TEMPLATE=/opt/bin/host-rkt
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
#!/bin/sh
# This is bind mounted into the kubelet rootfs and all rkt shell-outs go
# through this rkt wrapper. It essentially enters the host mount namespace
# (which it is already in) only for the purpose of breaking out of the chroot
# before calling rkt. It makes things like rkt gc work and avoids bind mounting
# in certain rkt filesystem dependancies into the kubelet rootfs. This can
# eventually be obviated when the write-api stuff gets upstream and rkt gc is
# through the api-server. Related issue:
# https://github.com/coreos/rkt/issues/2878
exec nsenter -m -u -i -n -p -t 1 -- /usr/bin/rkt "\$@"
EOF
    fi


    local TEMPLATE=$systemd_svc_dir/load-rkt-stage1.service
    if [ ${CONTAINER_RUNTIME} = "rkt" ] && [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Unit]
Description=Load rkt stage1 images
Documentation=http://github.com/coreos/rkt
Requires=network-online.target
After=network-online.target
Before=rkt-api.service

[Service]
RemainAfterExit=yes
Type=oneshot
ExecStart=/usr/bin/rkt fetch /usr/lib/rkt/stage1-images/stage1-coreos.aci /usr/lib/rkt/stage1-images/stage1-fly.aci  --insecure-options=image

[Install]
RequiredBy=rkt-api.service
EOF
    fi

    local TEMPLATE=$systemd_svc_dir/rkt-api.service
    if [ ${CONTAINER_RUNTIME} = "rkt" ] && [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Unit]
Before=kubelet.service

[Service]
ExecStart=/usr/bin/rkt api-service
Restart=always
RestartSec=10

[Install]
RequiredBy=kubelet.service
EOF
    fi

    local TEMPLATE=$k8s_manifests_dir/kube-proxy.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-proxy
  namespace: kube-system
  labels:
    k8s-app: kube-proxy
  annotations:
    rkt.alpha.kubernetes.io/stage1-name-override: coreos.com/rkt/stage1-fly
spec:
  hostNetwork: true
  containers:
  - name: kube-proxy
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
    command:
    - /proxy
    - --master=https://127.0.0.1:6443
    - --cluster-cidr=${POD_NETWORK}
    - --kubeconfig=/etc/kubernetes/kubeconfig/kube-proxy.yaml
    securityContext:
      privileged: true
    volumeMounts:
    - mountPath: /etc/kubernetes/kubeconfig/kube-proxy.yaml
      name: kubeconfig
      readOnly: true
    - mountPath: /etc/kubernetes/ssl/kube-proxy
      name: ssl-certs-kubernetes
      readOnly: true
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
    - mountPath: /var/run/dbus
      name: dbus
      readOnly: false
  volumes:
  - hostPath:
      path: /etc/kubernetes/kubeconfig/kube-proxy.yaml
    name: kubeconfig
  - hostPath:
      path: /etc/kubernetes/ssl/kube-proxy
    name: ssl-certs-kubernetes
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
  - hostPath:
      path: /var/run/dbus
    name: dbus
EOF
    fi

    local TEMPLATE=$k8s_manifests_dir/kube-apiserver.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
  labels:
    k8s-app: kube-apiserver
spec:
  hostNetwork: true
  containers:
  - name: kube-apiserver
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
    command:
    - /apiserver
    - --bind-address=0.0.0.0
    - --etcd-cafile=/etc/kubernetes/ssl/kube-apiserver/ca.pem
    - --etcd-certfile=/etc/kubernetes/ssl/kube-apiserver/client.pem
    - --etcd-keyfile=/etc/kubernetes/ssl/kube-apiserver/client-key.pem
    - --etcd-servers=${ETCD_ENDPOINTS}
    - --allow-privileged=true
    - --service-cluster-ip-range=${SERVICE_IP_RANGE}
    - --secure-port=6443
    - --insecure-port=8080
    - --advertise-address=${ADVERTISE_IP}
    - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota
    - --tls-ca-file=/etc/kubernetes/ssl/kube-apiserver/ca.pem
    - --tls-cert-file=/etc/kubernetes/ssl/kube-apiserver/server.pem
    - --tls-private-key-file=/etc/kubernetes/ssl/kube-apiserver/server-key.pem
    - --kubelet-certificate-authority=/etc/kubernetes/ssl/kube-apiserver/ca.pem
    - --client-ca-file=/etc/kubernetes/ssl/kube-apiserver/ca.pem
    - --service-account-key-file=/etc/kubernetes/ssl/kube-apiserver/serviceaccount-key.pem
    - --service-account-lookup
    - --runtime-config=extensions/v1beta1/networkpolicies=true
#    - --anonymous-auth=false
    - --authorization-mode=RBAC
    - --runtime-config=rbac.authorization.k8s.io/v1beta1
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        port: 6443
        scheme: HTTPS
        path: /healthz
      initialDelaySeconds: 15
      timeoutSeconds: 15
    ports:
    - containerPort: 6443
      hostPort: 6443
      name: https
    volumeMounts:
    - mountPath: /etc/kubernetes/ssl/kube-apiserver
      name: ssl-certs-kubernetes
      readOnly: true
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
  volumes:
  - hostPath:
      path: /etc/kubernetes/ssl/kube-apiserver
    name: ssl-certs-kubernetes
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
EOF
    fi

    local TEMPLATE=$k8s_manifests_dir/kube-controller-manager.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
  labels:
    k8s-app: kube-controller-manager
spec:
  containers:
  - name: kube-controller-manager
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
    command:
    - /controller-manager
    - --master=https://127.0.0.1:6443
    - --leader-elect=true
    - --use-service-account-credentials=true
    - --service-account-private-key-file=/etc/kubernetes/ssl/kube-apiserver/serviceaccount-key.pem
    - --root-ca-file=/etc/kubernetes/ssl/kube-controller-manager/ca.pem
    - --kubeconfig=/etc/kubernetes/kubeconfig/kube-controller-manager.yaml
    resources:
      requests:
        cpu: 200m
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10252
      initialDelaySeconds: 15
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /etc/kubernetes/kubeconfig/kube-controller-manager.yaml
      name: kubeconfig
      readOnly: true
    - mountPath: /etc/kubernetes/ssl/kube-controller-manager
      name: ssl-certs-kubernetes
      readOnly: true
    - mountPath: /etc/kubernetes/ssl/kube-apiserver/serviceaccount-key.pem
      name: ssl-certs-apiserver
      readOnly: true
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
  hostNetwork: true
  volumes:
  - hostPath:
      path: /etc/kubernetes/kubeconfig/kube-controller-manager.yaml
    name: kubeconfig
  - hostPath:
      path: /etc/kubernetes/ssl/kube-controller-manager
    name: ssl-certs-kubernetes
  - hostPath:
      path: /etc/kubernetes/ssl/kube-apiserver/serviceaccount-key.pem
    name: ssl-certs-apiserver
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
EOF
    fi

    local TEMPLATE=$k8s_manifests_dir/kube-scheduler.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
  labels:
    k8s-app: kube-scheduler
spec:
  hostNetwork: true
  containers:
  - name: kube-scheduler
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
    command:
    - /scheduler
    - --master=https://127.0.0.1:6443
    - --leader-elect=true
    - --kubeconfig=/etc/kubernetes/kubeconfig/kube-scheduler.yaml
    resources:
      requests:
        cpu: 100m
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10251
      initialDelaySeconds: 15
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /etc/kubernetes/kubeconfig/kube-scheduler.yaml
      name: kubeconfig
      readOnly: true
    - mountPath: /etc/kubernetes/ssl/kube-scheduler
      name: ssl-certs-kubernetes
      readOnly: true
  volumes:
  - hostPath:
      path: /etc/kubernetes/kubeconfig/kube-scheduler.yaml
    name: kubeconfig
  - hostPath:
      path: /etc/kubernetes/ssl/kube-scheduler
    name: ssl-certs-kubernetes
EOF
    fi

    local TEMPLATE=/etc/kubernetes/kubeconfig/kubelet.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    certificate-authority: /etc/kubernetes/ssl/kubelet/ca.pem
users:
- name: kubelet
  user:
    client-certificate: /etc/kubernetes/ssl/kubelet/client.pem
    client-key: /etc/kubernetes/ssl/kubelet/client-key.pem
contexts:
- context:
    cluster: local
    user: kubelet
  name: kubelet-context
current-context: kubelet-context
EOF
    fi

    local TEMPLATE=/etc/kubernetes/kubeconfig/kube-controller-manager.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    certificate-authority: /etc/kubernetes/ssl/kube-controller-manager/ca.pem
users:
- name: controller-manager
  user:
    client-certificate: /etc/kubernetes/ssl/kube-controller-manager/client.pem
    client-key: /etc/kubernetes/ssl/kube-controller-manager/client-key.pem
contexts:
- context:
    cluster: local
    user: controller-manager
  name: controller-manager-context
current-context: controller-manager-context
EOF
    fi

    local TEMPLATE=/etc/kubernetes/kubeconfig/kube-scheduler.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    certificate-authority: /etc/kubernetes/ssl/kube-scheduler/ca.pem
users:
- name: scheduler
  user:
    client-certificate: /etc/kubernetes/ssl/kube-scheduler/client.pem
    client-key: /etc/kubernetes/ssl/kube-scheduler/client-key.pem
contexts:
- context:
    cluster: local
    user: scheduler
  name: scheduler-context
current-context: scheduler-context
EOF
    fi

    local TEMPLATE=/etc/kubernetes/kubeconfig/kube-proxy.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    certificate-authority: /etc/kubernetes/ssl/kube-proxy/ca.pem
users:
- name: proxy
  user:
    client-certificate: /etc/kubernetes/ssl/kube-proxy/client.pem
    client-key: /etc/kubernetes/ssl/kube-proxy/client-key.pem
contexts:
- context:
    cluster: local
    user: proxy
  name: proxy-context
current-context: proxy-context
EOF
    fi

    local TEMPLATE=$manifests_dir/kube-dns-de.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
spec:
  strategy:
    rollingUpdate:
      maxSurge: 10%
      maxUnavailable: 0
  selector:
    matchLabels:
      k8s-app: kube-dns
  template:
    metadata:
      labels:
        k8s-app: kube-dns
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      serviceAccount: kube-dns
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
      containers:
      - name: kubedns
        image: gcr.io/google_containers/kubedns-amd64:1.9
        resources:
          limits:
            memory: 170Mi
          requests:
            cpu: 100m
            memory: 70Mi
        livenessProbe:
          httpGet:
            path: /healthz-kubedns
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
        readinessProbe:
          httpGet:
            path: /readiness
            port: 8081
            scheme: HTTP
          initialDelaySeconds: 3
          timeoutSeconds: 5
        args:
        - --domain=cluster.local.
        - --dns-port=10053
        - --v=2
        env:
        - name: PROMETHEUS_PORT
          value: "10055"
        ports:
        - containerPort: 10053
          name: dns-local
          protocol: UDP
        - containerPort: 10053
          name: dns-tcp-local
          protocol: TCP
        - containerPort: 10055
          name: metrics
          protocol: TCP
      - name: dnsmasq
        image: gcr.io/google_containers/kube-dnsmasq-amd64:1.4.1
        livenessProbe:
          httpGet:
            path: /healthz-dnsmasq
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
        args:
        - --cache-size=1000
        - --no-resolv
        - --server=127.0.0.1#10053
        - --log-facility=-
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        # see: https://github.com/kubernetes/kubernetes/issues/29055 for details
        resources:
          requests:
            cpu: 150m
            memory: 10Mi
      - name: dnsmasq-metrics
        image: gcr.io/google_containers/dnsmasq-metrics-amd64:1.0.1
        livenessProbe:
          httpGet:
            path: /metrics
            port: 10054
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
        args:
        - --v=2
        - --logtostderr
        ports:
        - containerPort: 10054
          name: metrics
          protocol: TCP
        resources:
          requests:
            memory: 10Mi
      - name: healthz
        image: gcr.io/google_containers/exechealthz-amd64:1.2
        resources:
          limits:
            memory: 50Mi
          requests:
            cpu: 10m
            memory: 50Mi
        args:
        - --cmd=nslookup kubernetes.default.svc.cluster.local 127.0.0.1 >/dev/null
        - --url=/healthz-dnsmasq
        - --cmd=nslookup kubernetes.default.svc.cluster.local 127.0.0.1:10053 >/dev/null
        - --url=/healthz-kubedns
        - --port=8080
        - --quiet
        ports:
        - containerPort: 8080
          protocol: TCP
      dnsPolicy: Default
EOF
    fi

    # Add the "get" verb to kube-dns' default ClusterRole policy
    # This is a temporary measure to accommodate the prerequisites of
    # kube-dns <1.6. See https://github.com/kubernetes/kubernetes/issues/45084
    # To be removed once the kube-dns binary is updated to v1.6
    local TEMPLATE=$manifests_dir/kube-dns-rbac.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
kind: ServiceAccount
apiVersion: v1
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    addonmanager.kubernetes.io/mode: Reconcile
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: system:kube-dns
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - services
  verbs:
  - get
  - list
  - watch
EOF
    fi

    local TEMPLATE=$manifests_dir/kube-dns-autoscaler-de.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-dns-autoscaler
  namespace: kube-system
  labels:
    k8s-app: kube-dns-autoscaler
    kubernetes.io/cluster-service: "true"
spec:
  template:
    metadata:
      labels:
        k8s-app: kube-dns-autoscaler
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      serviceAccount: kube-dns-autoscaler
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
      containers:
      - name: autoscaler
        image: gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.1.1-r2
        resources:
            requests:
                cpu: "20m"
                memory: "10Mi"
        command:
          - /cluster-proportional-autoscaler
          - --namespace=kube-system
          - --configmap=kube-dns-autoscaler
          - --target=Deployment/kube-dns
          - --default-params={"linear":{"coresPerReplica":256,"nodesPerReplica":16,"min":1}}
          - --logtostderr=true
          - --v=2
EOF
    fi

    # Taken from: https://github.com/kubernetes/kubernetes/blob/master/cluster/addons/dns-horizontal-autoscaler/dns-horizontal-autoscaler-rbac.yaml
    local TEMPLATE=$manifests_dir/kube-dns-autoscaler-rbac.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
kind: ServiceAccount
apiVersion: v1
metadata:
  name: kube-dns-autoscaler
  namespace: kube-system
  labels:
    addonmanager.kubernetes.io/mode: Reconcile
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:kube-dns-autoscaler
  labels:
    addonmanager.kubernetes.io/mode: Reconcile
rules:
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["list"]
  - apiGroups: [""]
    resources: ["replicationcontrollers/scale"]
    verbs: ["get", "update"]
  - apiGroups: ["extensions"]
    resources: ["deployments/scale", "replicasets/scale"]
    verbs: ["get", "update"]
# Remove the configmaps rule once below issue is fixed:
# kubernetes-incubator/cluster-proportional-autoscaler#16
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "create"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:kube-dns-autoscaler
  labels:
    addonmanager.kubernetes.io/mode: Reconcile
subjects:
  - kind: ServiceAccount
    name: kube-dns-autoscaler
    namespace: kube-system
roleRef:
  kind: ClusterRole
  name: system:kube-dns-autoscaler
  apiGroup: rbac.authorization.k8s.io
EOF
    fi

    local TEMPLATE=$manifests_dir/kube-dns-svc.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "KubeDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: ${DNS_SERVICE_IP}
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
EOF
    fi

    local TEMPLATE=$manifests_dir/heapster-de.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: heapster
  namespace: kube-system
  labels:
    k8s-app: heapster
    kubernetes.io/cluster-service: "true"
    version: v1.3.0
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: heapster
      version: v1.3.0
  template:
    metadata:
      labels:
        k8s-app: heapster
        version: v1.3.0
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      serviceAccount: heapster
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
      containers:
        - image: gcr.io/google_containers/heapster:v1.3.0
          name: heapster
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8082
              scheme: HTTP
            initialDelaySeconds: 180
            timeoutSeconds: 5
          command:
            - /heapster
            - --source=kubernetes.summary_api:''
        - image: gcr.io/google_containers/addon-resizer:1.7
          name: heapster-nanny
          resources:
            limits:
              cpu: 50m
              memory: 90Mi
            requests:
              cpu: 50m
              memory: 90Mi
          env:
            - name: MY_POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: MY_POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          command:
            - /pod_nanny
            - --cpu=80m
            - --extra-cpu=4m
            - --memory=200Mi
            - --extra-memory=4Mi
            - --threshold=5
            - --deployment=heapster
            - --container=heapster
            - --poll-period=300000
            - --estimator=exponential
EOF
    fi

    local TEMPLATE=$manifests_dir/heapster-svc.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
kind: Service
apiVersion: v1
metadata:
  name: heapster
  namespace: kube-system
  labels:
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "Heapster"
spec:
  ports:
    - port: 80
      targetPort: 8082
  selector:
    k8s-app: heapster
EOF
    fi

    # ClusterRole system:heapster is installed in k8s 1.6 by default, but
    # heapster-nanny needs get/update access to extensions/deployments
    local TEMPLATE=$manifests_dir/heapster-rbac.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: system:heapster
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:heapster
subjects:
- kind: ServiceAccount
  name: heapster
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: system:heapster
rules:
- apiGroups:
  - ""
  resources:
  - events
  - namespaces
  - nodes
  - pods
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - extensions
  resources:
  - deployments
  verbs:
  - get
  - update
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: heapster
  namespace: kube-system
EOF
    fi

    local TEMPLATE=$manifests_dir/kube-dashboard-de.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kubernetes-dashboard
  namespace: kube-system
  labels:
    k8s-app: kubernetes-dashboard
    kubernetes.io/cluster-service: "true"
spec:
  selector:
    matchLabels:
      k8s-app: kubernetes-dashboard
  template:
    metadata:
      labels:
        k8s-app: kubernetes-dashboard
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      serviceAccount: kubernetes-dashboard
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
      containers:
      - name: kubernetes-dashboard
        image: gcr.io/google_containers/kubernetes-dashboard-amd64:v1.6.0
        resources:
          # keep request = limit to keep this container in guaranteed class
          limits:
            cpu: 100m
            memory: 50Mi
          requests:
            cpu: 100m
            memory: 50Mi
        ports:
        - containerPort: 9090
        livenessProbe:
          httpGet:
            path: /
            port: 9090
          initialDelaySeconds: 30
          timeoutSeconds: 30
EOF
    fi

    local TEMPLATE=$manifests_dir/kube-dashboard-svc.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Service
metadata:
  name: kubernetes-dashboard
  namespace: kube-system
  labels:
    k8s-app: kubernetes-dashboard
    kubernetes.io/cluster-service: "true"
spec:
  selector:
    k8s-app: kubernetes-dashboard
  ports:
  - port: 80
    targetPort: 9090
EOF
    fi


    local TEMPLATE=$manifests_dir/kube-dashboard-rbac.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: kubernetes-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubernetes-dashboard
subjects:
- kind: ServiceAccount
  name: kubernetes-dashboard
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: kubernetes-dashboard
rules:
  - apiGroups:
      - "*"
    resources:
      - "*"
    verbs:
      - get
      - list
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubernetes-dashboard
  namespace: kube-system
EOF
    fi


    local TEMPLATE=/etc/flannel/options.env
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
RKT_RUN_ARGS=--uuid-file-save=/var/lib/coreos/flannel-wrapper.uuid --volume etc-flannel-ssl,kind=host,source=/etc/flannel/ssl,readOnly=true --mount volume=etc-flannel-ssl,target=/etc/flannel/ssl
FLANNELD_ETCD_CAFILE=/etc/flannel/ssl/ca.pem
FLANNELD_ETCD_CERTFILE=/etc/flannel/ssl/client.pem
FLANNELD_ETCD_KEYFILE=/etc/flannel/ssl/client-key.pem
FLANNELD_IFACE=$ADVERTISE_IP
FLANNELD_ETCD_ENDPOINTS=$ETCD_ENDPOINTS
EOF
    fi

    local TEMPLATE=$systemd_svc_dir/flanneld.service.d/40-ExecStartPre-symlink.conf
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
EOF
    fi

    local TEMPLATE=$systemd_svc_dir/docker.service.d/40-flannel.conf
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Unit]
Requires=flanneld.service
After=flanneld.service
[Service]
EnvironmentFile=/etc/kubernetes/cni/docker_opts_cni.env
EOF
    fi

    local TEMPLATE=/etc/kubernetes/cni/docker_opts_cni.env
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
DOCKER_OPT_BIP=""
DOCKER_OPT_IPMASQ=""
EOF
    fi

    local TEMPLATE=/etc/kubernetes/cni/net.d/10-flannel.conf
    if [ "${USE_CALICO}" = "false" ] && [ ! -f "${TEMPLATE}" ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
    "name": "podnet",
    "type": "flannel",
    "delegate": {
        "isDefaultGateway": true
    }
}
EOF
    fi

    # Config taken from http://docs.projectcalico.org/v2.1/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml
    local TEMPLATE=$manifests_dir/calico.yaml
    if [ "${USE_CALICO}" = "true" ]; then
      echo "TEMPLATE: $TEMPLATE"
      mkdir -p $(dirname $TEMPLATE)
      cat << EOF > $TEMPLATE
# This ConfigMap is used to configure a self-hosted Calico installation.
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config 
  namespace: kube-system
data:
  # Configure this with the location of your etcd cluster.
  etcd_endpoints: "${ETCD_ENDPOINTS}"
  # Configure this with the path of your etcd CA cert.
  etcd_ca_cert_file: "/etc/calico/ssl/ca.pem"
  etcd_cert_file: "/etc/calico/ssl/client.pem"
  etcd_key_file: "/etc/calico/ssl/client-key.pem"

  # The CNI network configuration to install on each node.  The special
  # values in this config will be automatically populated.
  cni_network_config: |-
    {
        "name": "calico",
        "type": "flannel",
        "delegate": {
          "type": "calico",
          "etcd_endpoints": "__ETCD_ENDPOINTS__",
          "etcd_ca_cert_file": "/etc/calico/ssl/ca.pem",
          "etcd_cert_file": "/etc/calico/ssl/client.pem",
          "etcd_key_file": "/etc/calico/ssl/client-key.pem",
          "log_level": "info",
          "policy": {
              "type": "k8s",
              "k8s_api_root": "https://__KUBERNETES_SERVICE_HOST__:__KUBERNETES_SERVICE_PORT__",
              "k8s_auth_token": "__SERVICEACCOUNT_TOKEN__"
          },
          "kubernetes": {
              "kubeconfig": "/etc/kubernetes/cni/net.d/__KUBECONFIG_FILENAME__"
          }
        }
    }

---

# This manifest installs the calico/node container, as well
# as the Calico CNI plugins and network config on 
# each master and worker node in a Kubernetes cluster.
kind: DaemonSet
apiVersion: extensions/v1beta1
metadata:
  name: calico-node
  namespace: kube-system
  labels:
    k8s-app: calico-node
spec:
  selector:
    matchLabels:
      k8s-app: calico-node
  template:
    metadata:
      labels:
        k8s-app: calico-node
      annotations:
        # Mark this pod as a critical add-on; when enabled, the critical add-on scheduler
        # reserves resources for critical add-on pods so that they can be rescheduled after
        # a failure.  This annotation works in tandem with the toleration below.
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      hostNetwork: true
      tolerations:
      - key: node-role.kubernetes.io/master
        effect: NoSchedule
      # Allow this pod to be rescheduled while the node is in "critical add-ons only" mode.
      # This, along with the annotation above marks this pod as a critical add-on.
      - key: CriticalAddonsOnly
        operator: Exists
      serviceAccount: calico-cni-plugin
      containers:
        # Runs calico/node container on each Kubernetes node.  This 
        # container programs network policy and routes on each
        # host.
        - name: calico-node
          image: quay.io/calico/node:v1.1.3
          env:
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # The path of the Calico etcd CA cert.
            - name: ETCD_CA_CERT_FILE
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_ca_cert_file
            # The path of the Calico etcd client cert.
            - name: ETCD_CERT_FILE
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_cert_file
            # The path of the Calico etcd client key.
            - name: ETCD_KEY_FILE
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_key_file
            # Choose the backend to use. 
            - name: CALICO_NETWORKING_BACKEND
              value: "none"
            # Disable file logging so 'kubectl logs' works.
            - name: CALICO_DISABLE_FILE_LOGGING
              value: "true"
            - name: NO_DEFAULT_POOLS
              value: "true"
          securityContext:
            privileged: true
          resources:
            requests:
              cpu: 250m
          volumeMounts:
            - mountPath: /etc/calico/ssl
              name: tls-certs
              readOnly: true
            - mountPath: /lib/modules
              name: lib-modules
              readOnly: false
            - mountPath: /var/run/calico
              name: var-run-calico
              readOnly: false
        # This container installs the Calico CNI binaries
        # and CNI network config file on each node.
        - name: install-cni
          image: quay.io/calico/cni:v1.7.0
          imagePullPolicy: Always
          command: ["/install-cni.sh"]
          env:
            # CNI configuration filename
            - name: CNI_CONF_NAME
              value: "10-calico.conf"
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # The path of the Calico etcd CA cert.
            - name: ETCD_CA_CERT_FILE
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_ca_cert_file
            # The CNI network config to install on each node.
            - name: CNI_NETWORK_CONFIG
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: cni_network_config
          volumeMounts:
            - mountPath: /etc/calico/ssl
              name: tls-certs
              readOnly: true
            - mountPath: /host/opt/cni/bin
              name: cni-bin-dir
            - mountPath: /host/etc/cni/net.d
              name: cni-net-dir
      volumes:
        - name: tls-certs
          hostPath:
            path: /etc/calico/ssl
        # Used by calico/node.
        - name: lib-modules
          hostPath:
            path: /lib/modules
        - name: var-run-calico
          hostPath:
            path: /var/run/calico
        # Used to install CNI.
        - name: cni-bin-dir
          hostPath:
            path: /opt/cni/bin
        - name: cni-net-dir
          hostPath:
            path: /etc/kubernetes/cni/net.d

---

# This manifest deploys the Calico policy controller on Kubernetes.
# See https://github.com/projectcalico/k8s-policy
apiVersion: extensions/v1beta1
kind: Deployment 
metadata:
  name: calico-policy-controller
  namespace: kube-system
  labels:
    k8s-app: calico-policy
spec:
  # The policy controller can only have a single active instance.
  replicas: 1
  strategy:
    type: Recreate
  template:
    metadata:
      name: calico-policy-controller
      namespace: kube-system
      labels:
        k8s-app: calico-policy-controller
      annotations:
        # Mark this pod as a critical add-on; when enabled, the critical add-on scheduler
        # reserves resources for critical add-on pods so that they can be rescheduled after
        # a failure.  This annotation works in tandem with the toleration below.
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      # The policy controller must run in the host network namespace so that
      # it isn't governed by policy that would prevent it from working.
      hostNetwork: true
      tolerations:
      - key: node-role.kubernetes.io/master
        effect: NoSchedule
      # Allow this pod to be rescheduled while the node is in "critical add-ons only" mode.
      # This, along with the annotation above marks this pod as a critical add-on.
      - key: CriticalAddonsOnly
        operator: Exists
      serviceAccount: calico-policy-controller
      containers:
        - name: calico-policy-controller
          image: quay.io/calico/kube-policy-controller:v0.6.0
          env:
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # The path of the Calico etcd CA cert.
            - name: ETCD_CA_CERT_FILE
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_ca_cert_file
            # The path of the Calico etcd client cert.
            - name: ETCD_CERT_FILE
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_cert_file
            # The path of the Calico etcd client key.
            - name: ETCD_KEY_FILE
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_key_file
            # The location of the Kubernetes API.  Use the default Kubernetes
            # service for API access.
            - name: K8S_API
              value: "https://kubernetes.default:443"
            # Since we're running in the host namespace and might not have KubeDNS 
            # access, configure the container's /etc/hosts to resolve
            # kubernetes.default to the correct service clusterIP.
            - name: CONFIGURE_ETC_HOSTS
              value: "true"
          volumeMounts:
            - mountPath: /etc/calico/ssl
              name: tls-certs
              readOnly: true
      volumes:
        # Used by both containers
        - name: tls-certs
          hostPath:
            path: /etc/calico/ssl
EOF
    fi

    local TEMPLATE=$manifests_dir/calico-rbac.yaml
    if [ "${USE_CALICO}" = "true" ]; then
      echo "TEMPLATE: $TEMPLATE"
      mkdir -p $(dirname $TEMPLATE)
      cat << EOF > $TEMPLATE
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: calico-cni-plugin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: calico-cni-plugin
subjects:
- kind: ServiceAccount
  name: calico-cni-plugin
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: calico-cni-plugin
  namespace: kube-system
rules:
  - apiGroups: [""]
    resources:
      - pods
      - nodes
    verbs:
      - get
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: calico-cni-plugin
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: calico-policy-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: calico-policy-controller
subjects:
- kind: ServiceAccount
  name: calico-policy-controller
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: calico-policy-controller
rules:
  - apiGroups:
    - ""
    - extensions
    resources:
      - pods
      - namespaces
      - networkpolicies
    verbs:
      - watch
      - list
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: calico-policy-controller
  namespace: kube-system
EOF
    fi

}

function wait_for_k8s_api {
    echo "Waiting for Kubernetes API..."
    until curl --silent "http://127.0.0.1:8080/version" > /dev/null
    do
        sleep 5
    done
}

function install_bridge_cni_plugin {
    echo
    echo "Install 'bridge' plugin needed by k8s CNI"
    docker run --rm --net=host -v /opt/cni/bin:/host/opt/cni/bin $HYPERKUBE_IMAGE_REPO:$K8S_VER cp /opt/cni/bin/bridge /host/opt/cni/bin/bridge
}

function start_addons {
    echo
    echo "Create Service Accounts"
    curl_cmd="curl --silent -H 'Content-Type: application/yaml' -XPOST"

    echo
    echo "K8S: DNS addon"
    eval "$curl_cmd --data-binary @$manifests_dir/kube-dns-de.yaml http://127.0.0.1:8080/apis/extensions/v1beta1/namespaces/kube-system/deployments > /dev/null"
    eval "$curl_cmd --data-binary @$manifests_dir/kube-dns-svc.yaml http://127.0.0.1:8080/api/v1/namespaces/kube-system/services > /dev/null"
    eval "$curl_cmd --data-binary @$manifests_dir/kube-dns-autoscaler-de.yaml http://127.0.0.1:8080/apis/extensions/v1beta1/namespaces/kube-system/deployments > /dev/null"
    docker run --rm --net=host -v $manifests_dir:/host/manifests $HYPERKUBE_IMAGE_REPO:$K8S_VER /kubectl apply -f /host/manifests/kube-dns-rbac.yaml -f /host/manifests/kube-dns-autoscaler-rbac.yaml
    echo "K8S: Heapster addon"
    eval "$curl_cmd --data-binary @$manifests_dir/heapster-de.yaml http://127.0.0.1:8080/apis/extensions/v1beta1/namespaces/kube-system/deployments > /dev/null"
    eval "$curl_cmd --data-binary @$manifests_dir/heapster-svc.yaml http://127.0.0.1:8080/api/v1/namespaces/kube-system/services > /dev/null"
    docker run --rm --net=host -v $manifests_dir:/host/manifests $HYPERKUBE_IMAGE_REPO:$K8S_VER /kubectl apply -f /host/manifests/heapster-rbac.yaml
    echo "K8S: Dashboard addon"
    eval "$curl_cmd --data-binary @$manifests_dir/kube-dashboard-de.yaml http://127.0.0.1:8080/apis/extensions/v1beta1/namespaces/kube-system/deployments > /dev/null"
    eval "$curl_cmd --data-binary @$manifests_dir/kube-dashboard-svc.yaml http://127.0.0.1:8080/api/v1/namespaces/kube-system/services > /dev/null"
    docker run --rm --net=host -v $manifests_dir:/host/manifests $HYPERKUBE_IMAGE_REPO:$K8S_VER /kubectl apply -f /host/manifests/kube-dashboard-rbac.yaml
}

function start_calico {
    echo "Deploying Calico"
    # Deploy Calico
    #TODO: change to rkt once this is resolved (https://github.com/coreos/rkt/issues/3181)
    docker run --rm --net=host -v $manifests_dir:/host/manifests $HYPERKUBE_IMAGE_REPO:$K8S_VER /kubectl apply -f /host/manifests/calico.yaml
    docker run --rm --net=host -v $manifests_dir:/host/manifests $HYPERKUBE_IMAGE_REPO:$K8S_VER /kubectl apply -f /host/manifests/calico-rbac.yaml
}

# kube-apiserver is started temporarily with the insecure port enabled.
# This is to facilitate the installation of add-ons such as kube-dns.
# Once the add-ons are installed, the insecure port is removed accordingly
function remove_apiserver_insecure_port {
    # Delete the line with the insecure port flag
    sed -i s/insecure-port=8080/insecure-port=0/ $k8s_manifests_dir/kube-apiserver.yaml

    # Restart kubelet because it won't automatically restart kube-apiserver after its manifest is changed
    systemctl restart kubelet
}

init_config
init_templates

chmod +x /opt/bin/host-rkt

init_flannel

systemctl stop update-engine; systemctl mask update-engine
systemctl daemon-reload

if [ $CONTAINER_RUNTIME = "rkt" ]; then
        systemctl enable load-rkt-stage1
        systemctl enable rkt-api
fi

systemctl enable flanneld; systemctl start flanneld
systemctl enable kubelet; systemctl start kubelet

wait_for_k8s_api
install_bridge_cni_plugin

if [ $USE_CALICO = "true" ]; then
        start_calico
fi

start_addons
remove_apiserver_insecure_port

echo "DONE"