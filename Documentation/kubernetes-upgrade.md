# Upgrading Kubernetes

This document describes upgrading the Kubernetes components on a cluster's master and worker nodes. For general information on Kubernetes cluster management, upgrades (including more advanced topics such as major API version upgrades) see the [Kubernetes upstream documentation](http://kubernetes.io/docs/admin/cluster-management.html) and [version upgrade notes](https://github.com/kubernetes/kubernetes/blob/release-1.2/docs/design/versioning.md#upgrades)

**NOTE:** The following upgrade documentation is for installations based on the CoreOS + Kubernetes step-by-step [installation guide](https://coreos.com/kubernetes/docs/latest/getting-started.html). Upgrade documentation for the AWS cloud-formation based installation is forthcoming.

## Upgrading the Kubelet

The Kubelet runs on both master and worker nodes, and is distributed as a hyperkube container image. The image version is usually set as an environment variable in the `kubelet.service` file, which is then passed to the [kubelet-wrapper](kubelet-wrapper.md) script.

To update the image version, modify the kubelet service file on each node (`/etc/systemd/system/kubelet.service`) to reference the new hyperkube image.

For example, modifying the `KUBELET_VERSION` environment variable in the following service file would change the container image version used when launching the kubelet via the [kubelet-wrapper](kubelet-wrapper.md) script.

**/etc/systemd/system/kubelet.service**

```
Environment=KUBELET_VERSION=v1.2.0_coreos.1
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=https://master [...]
```

## Upgrading Master Nodes

Master nodes consist of the following Kubernetes components:

* kube-proxy
* kube-apiserver
* kube-controller-manager
* kube-scheduler
* kube-podmaster (high-availability)

While upgrading the master components, user pods on worker nodes will continue to run normally.

### Upgrading kube-apiserver and kube-proxy

Both the kube-apiserver and kube-proxy are run as "static pods". This means the pod definition is a file on disk (default location: `/etc/kubernetes/manifests`). To update these components, you simply need to update the static manifest file. When the manifest changes on disk, the kubelet will pick up the changes and restart the local pod.

For example, to upgrade the kube-apiserver version you could update the pod image tag in `/etc/kubernetes/manifests/kube-apiserver.yaml`:

From: `image: quay.io/coreos/hyperkube:v1.0.6_coreos.0`
To: `image: quay.io/coreos//hyperkube:v1.0.7_coreos.0`

The kubelet would then restart the pod, and the new image version would be used.

**NOTE:** If you are running a multi-master high-availabililty cluster, please see the next section on upgrading the remaining master node components. Otherwise you can upgrade the remaining static pods (controller-manager, scheduler) using the same process described above.

### Upgrading Remaining Master Node Components (High-Availability)

The kube-controller-manager, kube-scheduler, and kube-podmaster are all also deployed as static pods in `/etc/kubernetes/manifests`. However, in high-availability deployments, the kube-podmaster is responsible for making sure only a single copy of the controller-manager and scheduler are running cluster-wide.

To accomplish this the kube-podmaster on each master node, if leader-elected, will copy the static manifest from `/srv/kubernetes/manifests` into `/etc/kubernetes/manifests` and the kubelet will pick up the manifest and run the pod. If the kube-podmaster loses its status as leader, it will remove the static pod from `/etc/kubernetes/manifests/` and the kubelet will shut down the pod.

This configuration means upgrading of these components will take a little more coordination.

To upgrade the kube-controller-manager and kube-scheduler:

1. For each master node:
   1. Make changes to the base manifests in `/srv/kubernetes/manifests`
   1. Remove the existing manifests (if present) from `/etc/kubernetes/manifests`
   1. The kube-podmaster will automatically fetch the new manifest from `/srv/kubernetes/manifests` and copy it to `/etc/kubernetes/manifests` and the new pod will be started.

**NOTE:** Because a particular master node may not be elected to run a particular component (e.g. kube-scheduler), updating the local manifest may not update the currently active instance of the Pod. You should update the manifests on all master nodes to ensure that no matter which is active, all will reflect the updated manifest.

### Upgrading Worker Nodes

Worker nodes will consist of the following kubernetes components.

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
