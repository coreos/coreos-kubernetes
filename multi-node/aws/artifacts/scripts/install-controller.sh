#!/bin/bash
set -e

# List of etcd servers (http://ip:port), comma separated
export ETCD_ENDPOINTS=

# Specify the version (vX.Y.Z) of Kubernetes assets to deploy
export K8S_VER=v1.1.2

# The CIDR network to use for pod IPs.
# Each pod launched in the cluster will be assigned an IP out of this range.
# Each node will be configured such that these IPs will be routable using the flannel overlay network.
export POD_NETWORK=

# The CIDR network to use for service cluster IPs.
# Each service will be assigned a cluster IP out of this range.
# This must not overlap with any IP ranges assigned to the POD_NETWORK, or other existing network infrastructure.
# Routing to these IPs is handled by a proxy service local to each node, and are not required to be routable between nodes.
export SERVICE_IP_RANGE=

# The IP address of the Kubernetes API Service
# If the SERVICE_IP_RANGE is changed above, this must be set to the first IP in that range.
export K8S_SERVICE_IP=

# The IP address of the cluster DNS service.
# This IP must be in the range of the SERVICE_IP_RANGE and cannot be the first IP in the range.
# This same IP must be configured on all worker nodes to enable DNS service discovery.
export DNS_SERVICE_IP=

# The HTTP(S) host serving the necessary Kubernetes artifacts
export ARTIFACT_URL=

# The above settings can optionally be overridden using an environment file:
ENV_FILE=/run/coreos-kubernetes/options.env

# -------------

function template {
	# use a heredoc so the quoting & whitespace in the
	# downloaded artifact is preserved, but env variables
	# can still be evaluated
	eval "cat <<EOF
$(curl --silent -L "${ARTIFACT_URL}/$1")
EOF
" > $2
}

function init_config {
	local REQUIRED=('ADVERTISE_IP' 'POD_NETWORK' 'ETCD_ENDPOINTS' 'SERVICE_IP_RANGE' 'K8S_SERVICE_IP' 'DNS_SERVICE_IP' 'K8S_VER' 'ARTIFACT_URL' )

	if [ -f $ENV_FILE ]; then
		export $(cat $ENV_FILE | xargs)
	fi

	if [ -z $ADVERTISE_IP ]; then
		export ADVERTISE_IP=$(awk -F= '/COREOS_PRIVATE_IPV4/ {print $2}' /etc/environment)
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
	[ -f $TEMPLATE ] || {
		echo "TEMPLATE: $TEMPLATE"
		mkdir -p $(dirname $TEMPLATE)
		cat << EOF > $TEMPLATE
[Service]
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
ExecStart=/usr/bin/kubelet \
  --api_servers=http://127.0.0.1:8080 \
  --register-node=false \
  --allow-privileged=true \
  --config=/etc/kubernetes/manifests \
  --cluster_dns=${DNS_SERVICE_IP} \
  --cluster_domain=cluster.local
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
	}

	mkdir -p /etc/kubernetes/manifests
	template manifests/controller/kube-proxy.yaml /etc/kubernetes/manifests/kube-proxy.yaml
	template manifests/controller/kube-apiserver.yaml /etc/kubernetes/manifests/kube-apiserver.yaml
	template manifests/controller/kube-podmaster.yaml /etc/kubernetes/manifests/kube-podmaster.yaml

	mkdir -p /srv/kubernetes/manifests
	template manifests/controller/kube-controller-manager.yaml /srv/kubernetes/manifests/kube-controller-manager.yaml
	template manifests/controller/kube-scheduler.yaml /srv/kubernetes/manifests/kube-scheduler.yaml

	template manifests/cluster/kube-system.json /srv/kubernetes/manifests/kube-system.json
	template manifests/cluster/kube-dns-rc.json /srv/kubernetes/manifests/kube-dns-rc.json
	template manifests/cluster/kube-dns-svc.json /srv/kubernetes/manifests/kube-dns-svc.json
}

function start_addons {
	echo "Waiting for Kubernetes API..."
	until curl --silent "http://127.0.0.1:8080/version"
	do
		sleep 5
	done
	echo
	echo "K8S: kube-system namespace"
	curl --silent -XPOST -d"$(cat /srv/kubernetes/manifests/kube-system.json)" "http://127.0.0.1:8080/api/v1/namespaces" > /dev/null
	echo "K8S: DNS addon"
	curl --silent -XPOST -d"$(cat /srv/kubernetes/manifests/kube-dns-rc.json)" "http://127.0.0.1:8080/api/v1/namespaces/kube-system/replicationcontrollers" > /dev/null
	curl --silent -XPOST -d"$(cat /srv/kubernetes/manifests/kube-dns-svc.json)" "http://127.0.0.1:8080/api/v1/namespaces/kube-system/services" > /dev/null
}

init_config
init_templates

systemctl daemon-reload
systemctl stop update-engine; systemctl mask update-engine
echo "REBOOT_STRATEGY=off" >> /etc/coreos/update.conf

systemctl enable kubelet; systemctl start kubelet
start_addons
echo "DONE"
