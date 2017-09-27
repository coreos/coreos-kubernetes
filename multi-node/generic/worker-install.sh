#!/bin/bash
set -e

# IP address of this node
export ADVERTISE_IP=10.0.0.10

# List of etcd servers (http://ip:port), comma separated
export ETCD_ENDPOINTS=https://10.0.0.1:2379,https://10.0.0.2:2379,https://10.0.0.3:2379

# The endpoint the worker node should use to contact controller nodes (https://ip:port)
# For HA, this should either be an external DNS record or a loadbalancer pod in the worker node.
# It's also possible to point directly to a single control node.
export CONTROLLER_ENDPOINT=https://con.kube:6443

# Specify the version (vX.Y.Z) of Kubernetes assets to deploy
export K8S_VER=v1.6.2_coreos.0

# Hyperkube image repository to use.
export HYPERKUBE_IMAGE_REPO=quay.io/coreos/hyperkube

# The CIDR network to use for pod IPs.
# Each pod launched in the cluster will be assigned an IP out of this range.
# Each node will be configured such that these IPs will be routable using the flannel overlay network.
export POD_NETWORK=10.2.0.0/16

# The IP address of the cluster DNS service.
# This must be the same DNS_SERVICE_IP used when configuring the controller nodes.
export DNS_SERVICE_IP=10.3.0.10

# Whether to use Calico for Kubernetes network policy.
export USE_CALICO=true

# Determines the container runtime for kubernetes to use. Accepts 'docker' or 'rkt'.
export CONTAINER_RUNTIME=docker

# The above settings can optionally be overridden using an environment file:
ENV_FILE=/run/coreos-kubernetes/options.env

# To run a self hosted Calico install it needs to be able to write to the CNI dir
if [ "${USE_CALICO}" = "true" ]; then
    export CALICO_OPTS="--volume cni-bin,kind=host,source=/opt/cni/bin \\
  --mount volume=cni-bin,target=/opt/cni/bin"
else
    export CALICO_OPTS=""
fi

# -------------

function init_config {
    local REQUIRED=( 'ADVERTISE_IP' 'ETCD_ENDPOINTS' 'CONTROLLER_ENDPOINT' 'DNS_SERVICE_IP' 'K8S_VER' 'HYPERKUBE_IMAGE_REPO' 'USE_CALICO' )

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

function init_templates {
    local TEMPLATE=/etc/systemd/system/kubelet.service
    local uuid_file="/var/run/kubelet-pod.uuid"
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
Environment=KUBELET_IMAGE_TAG=${K8S_VER}
Environment=KUBELET_IMAGE_URL=${HYPERKUBE_IMAGE_REPO}
Environment="RKT_RUN_ARGS=--uuid-file-save=${uuid_file} \\
  --volume tls-k8s,kind=host,source=/etc/kubernetes/ssl/kubelet \\
  --mount volume=tls-k8s,target=/etc/kubernetes/ssl/kubelet \\
  --volume tls-calico,kind=host,source=/etc/calico/ssl \\
  --mount volume=tls-calico,target=/etc/calico/ssl \\
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
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
ExecStartPre=/usr/bin/mkdir -p /opt/cni/bin
ExecStartPre=/usr/bin/mkdir -p /var/log/containers
ExecStartPre=-/usr/bin/rkt rm --uuid-file=${uuid_file}
ExecStart=/usr/lib/coreos/kubelet-wrapper \\
  --client-ca-file=/etc/kubernetes/ssl/kubelet/ca.pem \\
  --tls-cert-file=/etc/kubernetes/ssl/kubelet/server.pem \\
  --tls-private-key-file=/etc/kubernetes/ssl/kubelet/server-key.pem \\
  --kubeconfig=/etc/kubernetes/kubeconfig/kubelet.yaml \\
  --api-servers=${CONTROLLER_ENDPOINT} \\
  --network-plugin=cni \\
  --network-plugin-dir=/etc/kubernetes/cni/net.d \\
  --cni-bin-dir=/opt/cni/bin \\
  --container-runtime=${CONTAINER_RUNTIME} \\
  --rkt-path=/usr/bin/rkt \\
  --rkt-stage1-image=coreos.com/rkt/stage1-coreos \\
  --register-node=true \\
  --allow-privileged=true \\
  --pod-manifest-path=/etc/kubernetes/manifests \\
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
exec nsenter -m -u -i -n -p -t 1 -- /usr/bin/rkt "\\$@"
EOF
    fi

    local TEMPLATE=/etc/systemd/system/load-rkt-stage1.service
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
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/bin/rkt fetch /usr/lib/rkt/stage1-images/stage1-coreos.aci /usr/lib/rkt/stage1-images/stage1-fly.aci  --insecure-options=image

[Install]
RequiredBy=rkt-api.service
EOF
    fi

    local TEMPLATE=/etc/systemd/system/rkt-api.service
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

    local TEMPLATE=/etc/kubernetes/manifests/kube-proxy.yaml
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-proxy
  namespace: kube-system
  annotations:
    rkt.alpha.kubernetes.io/stage1-name-override: coreos.com/rkt/stage1-fly
spec:
  hostNetwork: true
  containers:
  - name: kube-proxy
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
    command:
    - /hyperkube
    - proxy
    - --master=${CONTROLLER_ENDPOINT}
    - --cluster-cidr=${POD_NETWORK}
    - --kubeconfig=/etc/kubernetes/kubeconfig/kube-proxy.yaml
    securityContext:
      privileged: true
    volumeMounts:
    - mountPath: /etc/ssl/certs
      name: "ssl-certs"
    - mountPath: /etc/kubernetes/kubeconfig/kube-proxy.yaml
      name: "kubeconfig"
      readOnly: true
    - mountPath: /etc/kubernetes/ssl/kube-proxy
      name: "etc-kube-ssl"
      readOnly: true
    - mountPath: /var/run/dbus
      name: dbus
      readOnly: false
  volumes:
  - name: "ssl-certs"
    hostPath:
      path: "/usr/share/ca-certificates"
  - name: "kubeconfig"
    hostPath:
      path: "/etc/kubernetes/kubeconfig/kube-proxy.yaml"
  - name: "etc-kube-ssl"
    hostPath:
      path: "/etc/kubernetes/ssl/kube-proxy"
  - hostPath:
      path: /var/run/dbus
    name: dbus
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

    local TEMPLATE=/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
EOF
    fi

    local TEMPLATE=/etc/systemd/system/docker.service.d/40-flannel.conf
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

}

function install_bridge_cni_plugin {
    echo
    echo "Install 'bridge' plugin needed by k8s CNI"
    docker run --rm --net=host -v /opt/cni/bin:/host/opt/cni/bin $HYPERKUBE_IMAGE_REPO:$K8S_VER cp /opt/cni/bin/bridge /host/opt/cni/bin/bridge
}

init_config
init_templates

chmod +x /opt/bin/host-rkt

systemctl stop update-engine; systemctl mask update-engine

systemctl daemon-reload

if [ $CONTAINER_RUNTIME = "rkt" ]; then
        systemctl enable load-rkt-stage1
        systemctl enable rkt-api
fi

install_bridge_cni_plugin

systemctl enable flanneld; systemctl start flanneld
systemctl enable kubelet; systemctl start kubelet
