#!/bin/bash
set -e

# Advertised IP address of this node
export ADVERTISE_IP=10.0.0.1

# List of etcd servers (nodename=https://ip:port), comma separated
export ETCD_ENDPOINTS="con0=https://10.0.0.1:2380,con1=https://10.0.0.2:2380,con2=https://10.0.0.3:2380"

# Specify the version (vX.Y.Z) of etcd assets to deploy
export ETCD_VER=v3.1.5


function init_config {
    local REQUIRED=( 'ADVERTISE_IP' 'ETCD_ENDPOINTS' 'ETCD_VER' )

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
    local TEMPLATE=/etc/systemd/system/etcd-member.service.d/override.conf
    if [ ! -f $TEMPLATE ]; then
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
Environment="RKT_RUN_ARGS=\
  --uuid-file-save=/var/lib/coreos/etcd-member-wrapper.uuid \\
  --volume etcd-tls,kind=host,source=/etc/etcd/ssl \\
  --mount volume=etcd-tls,target=/etc/etcd/ssl"
Environment="ETCD_IMAGE_TAG=${ETCD_VER}"
Environment="ETCD_NAME=$(hostname | egrep -o '^[^\.]+')"
Environment="ETCD_OPTS=--trusted-ca-file=/etc/etcd/ssl/ca.pem \\
  --cert-file=/etc/etcd/ssl/peer.pem \\
  --key-file=/etc/etcd/ssl/peer-key.pem \\
  --client-cert-auth \\
  --peer-trusted-ca-file=/etc/etcd/ssl/ca.pem \\
  --peer-cert-file=/etc/etcd/ssl/peer.pem \\
  --peer-key-file=/etc/etcd/ssl/peer-key.pem \\
  --initial-advertise-peer-urls https://${ADVERTISE_IP}:2380 \\
  --listen-peer-urls https://${ADVERTISE_IP}:2380 \\
  --listen-client-urls https://${ADVERTISE_IP}:2379,http://127.0.0.1:2379 \\
  --advertise-client-urls https://${ADVERTISE_IP}:2379 \\
  --initial-cluster-token etcd-cluster-0 \\
  --initial-cluster ${ETCD_ENDPOINTS} \\
  --initial-cluster-state new"
EOF
    fi
}

## Check config and generate templates
echo "+ Config etcd"
init_config
init_templates
echo

## Enable etcd-member.service
echo "+ Start etcd"
systemctl daemon-reload
systemctl enable etcd-member.service
systemctl start etcd-member.service
echo

## Finished!
echo "DONE"