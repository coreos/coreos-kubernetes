#!/bin/bash
set -euo pipefail

CHECK_NODE_COUNT=${CHECK_NODE_COUNT:-true}
CONFORMANCE_REPO=${CONFORMANCE_REPO:-github.com/coreos/kubernetes}
CONFORMANCE_VERSION=${CONFORMANCE_VERSION:-v1.4.1+coreos.0}
SSH_OPTS=${SSH_OPTS:-}

usage() {
    echo "USAGE:"
    echo "  $0 <ssh-host> <ssh-port> <ssh-key>"
    echo
    exit 1
}

if [ $# -ne 3 ]; then
    usage
    exit 1
fi

ssh_host=$1
ssh_port=$2
ssh_key=$3

kubeconfig=$(cat <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://127.0.0.1:8080
EOF
)

K8S_SRC=/home/core/go/src/k8s.io/kubernetes
ssh ${SSH_OPTS} -i ${ssh_key} -p ${ssh_port} core@${ssh_host} \
    "mkdir -p ${K8S_SRC} && [[ -d ${K8S_SRC}/.git ]] || git clone https://${CONFORMANCE_REPO} ${K8S_SRC}"

ssh ${SSH_OPTS} -i ${ssh_key} -p ${ssh_port} core@${ssh_host} \
    "[[ -f /home/core/kubeconfig ]] || echo '${kubeconfig}' > /home/core/kubeconfig"

# Init steps necessary to run conformance in docker://golang:1.6.2 container
INIT="apt-get update && apt-get install -y rsync"

TEST_FLAGS="-v --test -check_version_skew=false -check_node_count=${CHECK_NODE_COUNT} --test_args=\"ginkgo.focus='\[Conformance\]'\""

CONFORMANCE=$(echo \
    "cd /go/src/k8s.io/kubernetes && " \
    "git checkout ${CONFORMANCE_VERSION} && " \
    "make all WHAT=cmd/kubectl && " \
    "make all WHAT=vendor/github.com/onsi/ginkgo/ginkgo && " \
    "make all WHAT=test/e2e/e2e.test && " \
    "KUBECONFIG=/kubeconfig KUBERNETES_PROVIDER=skeleton KUBERNETES_CONFORMANCE_TEST=Y go run hack/e2e.go ${TEST_FLAGS}")

RKT_OPTS=$(echo \
    "--volume=kc,kind=host,source=/home/core/kubeconfig "\
    "--volume=k8s,kind=host,source=${K8S_SRC} " \
    "--mount volume=kc,target=/kubeconfig " \
    "--mount volume=k8s,target=/go/src/k8s.io/kubernetes")

CMD="sudo rkt run --net=host --insecure-options=image ${RKT_OPTS} docker://golang:1.6.2 --exec /bin/bash -- -c \"${INIT} && ${CONFORMANCE}\""

ssh ${SSH_OPTS} -i ${ssh_key} -p ${ssh_port} core@${ssh_host} "${CMD}"
