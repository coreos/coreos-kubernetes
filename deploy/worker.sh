#!/bin/bash -e

K8S_VER="v1.0.3"

function usage {
    echo "USAGE: $0 <command>"
    echo "Commands:"
    echo -e "\tinit \tInitialize worker node services"
    echo -e "\tstart \tStart worker node services"
}

if [ -z $1 ]; then
    usage
    exit 1
fi

CMD=$1

function init_config {
    local REQUIRED=( 'ADVERTISE_IP' 'ETCD_ENDPOINTS' 'CONTROLLER_ENDPOINT' 'DNS_SERVICE_IP' )

    if [ -z $ADVERTISE_IP ]; then
        export ADVERTISE_IP=$(awk -F= '/COREOS_PUBLIC_IPV4/ {print $2}' /etc/environment)
    fi
    if [ -z $ETCD_ENDPOINTS ]; then
        export ETCD_ENDPOINTS="http://127.0.0.1:2379"
    fi
    if [ -z $CONTROLLER_ENDPOINT ]; then
        export CONTROLLER_ENDPOINT="http://127.0.0.1:8080"
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
}

function init_kubernetes_release {
    local RELEASE_DIR=/opt/kubernetes_release/$K8S_VER
    mkdir -p $RELEASE_DIR

    [ -f $RELEASE_DIR/kubernetes.tar.gz ] || {
        echo "K8S: downloading release: $K8S_VER"
        curl --silent -o $RELEASE_DIR/kubernetes.tar.gz https://storage.googleapis.com/kubernetes-release/release/$K8S_VER/kubernetes.tar.gz
    }

    [ -d $RELEASE_DIR/kubernetes ] || {
        echo "K8S: extracting release: $K8S_VER"
        tar xzf $RELEASE_DIR/kubernetes.tar.gz -C $RELEASE_DIR
    }

    [ -d $RELEASE_DIR/kubernetes/server/kubernetes ] || {
        echo "K8S: extracting server components: $K8S_VER"
        tar xzf $RELEASE_DIR/kubernetes/server/kubernetes-server-linux-amd64.tar.gz -C $RELEASE_DIR/kubernetes/server
    }

    mkdir -p /opt/bin
    BINS=( "kubectl" "kubelet" "kube-proxy" )
    for BIN in "${BINS[@]}"; do
      [ -x /opt/bin/$BIN ] || {
        echo "K8S BIN: $BIN"
        cp $RELEASE_DIR/kubernetes/server/kubernetes/server/bin/$BIN /opt/bin/$BIN
        chown core:core /opt/bin/$BIN
      }
    done
}

function init_templates {
    local TEMPLATE=/etc/systemd/system/kubelet.service
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Unit]
Requires=flanneld.service
After=flanneld.service

[Service]
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
ExecStart=/opt/bin/kubelet \
  --api_servers=${CONTROLLER_ENDPOINT} \
  --hostname-override=${ADVERTISE_IP} \
  --cluster_dns=${DNS_SERVICE_IP} \
  --kubeconfig=/srv/kubernetes/istv-kubeconfig.yaml \
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
[Unit]
Requires=flanneld.service
After=flanneld.service

[Service]
ExecStart=/opt/bin/kube-proxy \
  --master=${CONTROLLER_ENDPOINT} \
  --kubeconfig=/srv/kubernetes/istv-kubeconfig.yaml \
  --logtostderr
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    }

  local TEMPLATE=/srv/kubernetes/istv-kubeconfig.yaml
  [ -f $TEMPLATE ] || {
      echo "TEMPLATE: $TEMPLATE"
      mkdir -p $(dirname $TEMPLATE)
      cat << EOF > $TEMPLATE
apiVersion: v1
kind: Config
clusters:
- name: control
  cluster:
     insecure-skip-tls-verify: true
contexts:
- context:
    cluster: control
  name: default-context
current-context: default-context
EOF
    }

  local TEMPLATE=/home/core/.kube/config
  [ -f $TEMPLATE ] || {
      echo "TEMPLATE: $TEMPLATE"
      mkdir -p $(dirname $TEMPLATE)
      cat << EOF > $TEMPLATE
apiVersion: v1
kind: Config
clusters:
- name: default
  cluster:
    server: ${CONTROLLER_ENDPOINT}
    insecure-skip-tls-verify: true
users:
- name: core
  user:
    username: core
    password: core
contexts:
- name: default-context
  context:
    cluster: default
    user: core
current-context: default-context
EOF
    }
}

if [ "$CMD" == "init" ]; then
    echo "Starting initialization"
    init_config
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
    echo "Service start complete"
    exit 0
fi

usage
exit 1

