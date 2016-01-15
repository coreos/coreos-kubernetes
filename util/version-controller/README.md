# Kubelet Version Controller

The purpose of this tool is to provide a reference implementation of a tool which manages the kubelet versions running on CoreOS nodes.

## Overview

Each CoreOS node will be running a kubelet-agent service which is watching the kubernetes API for the version it is expected to run. How the expected version is decided upon may change in various deployments. However, this tool takes a simple approach of deploying rolling updates to kubelets after the master-components have been updated.

- Monitor the Kubernetes api-server version
- Continually drive convergence of the kubelet version to match api-server version.
- If too many nodes are in an updating state, or have not returned to "Node Ready" state, halt version updates.

## Example Process

master = v1.1.1
workerA = v1.1.1
workerB = v1.1.1

1. The version-controller is deployed using `--max-updates=1 --max-not-ready=1`.
1. An administrator updates the master-components to `v1.1.2`.
1. The version-controller begins updating Kubernets Node objects:
    1. Annotate node workerA: `coreos.com/expected-kubelet-version=v1.1.2`.
1. The node-agent on workerA sees the new expected version annotation.
    1. agent reboots kubelet to use updated v1.1.2 version.
    1. node posts `NodeReady=True` and `KubeletVersion=v1.1.2` to Kubernetes API.
1. The version-controller sees that the node has successfully updated and moves on to workerB.

# Deploy to Kubernetes

```
kubectl create -f version-controller-rc.yaml
```

# Development Build

```
make bin

# Optionally build container / push to development registry
make container PREFIX=<registry-prefix> TAG=<version>
make push PREFIX=<registry-prefix> TAG=<version>
```

# Build Release

1. Update TAG in Makefile
1. Open pull-request
1. Build / Push release:

    ```
    make release
    make push
    ```
