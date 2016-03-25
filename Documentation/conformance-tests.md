# Kubernetes Conformance Tests

The conformance test checks that a kubernetes installation supports a minimum required feature set.

## Prerequisites

You will need to have a Kubernetes cluster already running, and a kubeconfig with the current-context set to the cluster you wish to test.

Make sure only the essential pods are running, and there are no failed or pending pods. If you are testing a small / development cluster, you may need to increase the node memory or tests could inconsistently fail (e.g. 2048mb for single-node vagrant).


## Running the Tests

Follow these steps to run the conformance test against the desired Kubernetes release:

### Clone Kubernetes

First, clone the Kubernetes codebase:

```sh
$ git clone https://github.com/kubernetes/kubernetes.git
```

### Checkout Branch

Next, checkout the branch or release you'd like to test against:

```sh
$ cd kubernetes
$ git checkout v1.2.0
```

### Create Kubernetes Binaries

Build binaries from the codebase:

```sh
$ make clean
$ make quick-release
```

### Set Worker Count

Modify the `WORKERS` count to match the deployment you are testing:

```sh
$ WORKERS=1; sed -i '' "s/NUM_MINIONS=[0-9]/NUM_MINIONS=${WORKERS}/" hack/conformance-test.sh
```

### Run Conformance Tests

The command below expects a kubeconfig with the current context set to the cluster you wish to test. To set the path, update `KUBECONFIG` in the command below.

```sh
$ KUBECONFIG=$HOME/.kube/config hack/conformance-test.sh 2>&1 | tee conformance.$(date +%FT%T%z).log
```

**NOTE:** In single-node installations the test `should function for intra-pod communication`, will not pass because there are no additional workers to communicate with.
