# Upgrading Kubernetes

This document describes upgrading the Kubernetes components on a cluster's master and worker nodes. For general information on Kubernetes cluster management, upgrades (including more advanced topics such as major API version upgrades) see the [Kubernetes upstream documentation](http://kubernetes.io/docs/admin/cluster-management.html) and [version upgrade notes](https://github.com/kubernetes/kubernetes/blob/release-1.4/docs/design/versioning.md#upgrades)

**NOTE:** The following upgrade documentation is for installations based on the CoreOS + Kubernetes step-by-step [installation guide](https://coreos.com/kubernetes/docs/latest/getting-started.html). Upgrade documentation for the AWS cloud-formation based installation is forthcoming.

## Upgrading the Kubelet

The Kubelet runs on both master and worker nodes, and is distributed as a hyperkube container image. The image version is usually set as an environment variable in the `kubelet.service` file, which is then passed to the [kubelet-wrapper](kubelet-wrapper.md) script.

To update the image version, modify the kubelet service file on each node (`/etc/systemd/system/kubelet.service`) to reference the new hyperkube image.

For example, modifying the `KUBELET_VERSION` environment variable in the following service file would change the container image version used when launching the kubelet via the [kubelet-wrapper](kubelet-wrapper.md) script.

**/etc/systemd/system/kubelet.service**

```
Environment=KUBELET_VERSION=v1.4.1_coreos.0
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=https://master [...]
```

## Upgrading Calico

The Calico agent runs on both master and worker nodes, and is is distributed as a container image. It runs under rkt using systemd.

To update the image version, change the image tag in the service file (`/etc/systemd/system/calico-node.service`) to reference the new calico-node image.


**/etc/systemd/system/calico-node.service**

```
ExecStart=/usr/bin/rkt run --inherit-env --stage1-from-dir=stage1-fly.aci \
--volume=modules,kind=host,source=/lib/modules,readOnly=false \
--mount=volume=modules,target=/lib/modules \
--volume=dns,kind=host,source=/etc/resolv.conf,readOnly=true \
--mount=volume=dns,target=/etc/resolv.conf \
--trust-keys-from-https quay.io/calico/node:v0.19.0
```

## Upgrading Master Nodes

Master nodes consist of the following Kubernetes components:

* kube-proxy
* kube-apiserver
* kube-controller-manager
* kube-scheduler
* policy-controller

While upgrading the master components, user pods on worker nodes will continue to run normally.

### Upgrading Master Node Components

The master node components (kube-controller-manager,kube-scheduler, kube-apiserver, and kube-proxy) are run as "static pods". This means the pod definition is a file on disk (default location: `/etc/kubernetes/manifests`). To update these components, you simply need to update the static manifest file. When the manifest changes on disk, the kubelet will pick up the changes and restart the local pod.

For example, to upgrade the kube-apiserver version you could update the pod image tag in `/etc/kubernetes/manifests/kube-apiserver.yaml`:

From: `image: quay.io/coreos/hyperkube:v1.0.6_coreos.0`

To: `image: quay.io/coreos/hyperkube:v1.0.7_coreos.0`

In high-availability deployments, the control-plane components (apiserver, scheduler, and controller-manager) are deployed to all master nodes. Upgrades of these components will require them being updated on each master node.

**NOTE:** Because a particular master node may not be elected to run a particular component (e.g. kube-scheduler), updating the local manifest may not update the currently active instance of the Pod. You should update the manifests on all master nodes to ensure that no matter which is active, all will reflect the updated manifest.

### Upgrading Worker Nodes

Worker nodes consist of the following kubernetes components.

* kube-proxy

### Upgrading the kube-proxy

The kube-proxy is run as a "static pod". To upgrade the pod definition, simply modify the pod manifest located in `/etc/kubernetes/manifests/kube-proxy.yaml`. The kubelet will pick up the changes and re-launch the kube-proxy pod.

## Example Upgrade Process:

1. Prepare new pod manifests for master nodes
1. Prepare new pod manifests for worker nodes
1. For each master node:
    1. Back up existing manifests
    1. Update manifests
1. Repeat item 3 for each worker node
