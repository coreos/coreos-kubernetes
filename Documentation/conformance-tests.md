# Kubernetes Conformance Tests

The conformance test checks that a kubernetes installation supports a minimum required feature set.

## Kubernetes Cluster

You will need to have a cluster already running, and a kubeconfig with the current-context set to the cluster you wish to test.

Make sure only the essential pods are running, and there are no failed or pending pods. If you are testing a small / development cluster, you may need to increase the node memory or tests could inconsistently fail (e.g. 2048mb for single-node vagrant).

## Clone Kubernetes

```sh
git clone https://github.com/kubernetes/kubernetes.git
```

## Checkout Conformance Test branch

```sh
cd kubernetes
git checkout conformance-test-v1
```

## Create Kubernetes Binaries

```sh
make clean
make quick-release
```

## Set Worker Count

Modify the `WORKERS` count to match the deployment you are testing.

```sh
WORKERS=1; sed -i '' "s/NUM_MINIONS=[0-9]/NUM_MINIONS=${WORKERS}/" hack/conformance-test.sh
```

## Run Conformance Tests

The command below expects a kubeconfig with the current context set to the cluster you wish to test. To set the path, update `KUBECONFIG` in the command below.

```sh
KUBECONFIG=$HOME/.kube/config hack/conformance-test.sh 2>&1 | tee conformance.$(date +%FT%T%z).log
```

**NOTE:** In single-node installations the test `should function for intra-pod communication`, will not pass because there are no additional workers to communicate with.

