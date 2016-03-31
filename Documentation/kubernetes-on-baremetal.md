# Kubernetes Installation on Bare Metal &amp; CoreOS

This guide walks a deployer through launching a multi-node Kubernetes cluster on bare metal servers running CoreOS.
After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the kubectl CLI tool.

## Deployment Prerequisites

### CoreOS Installation

For all nodes running Kubernetes components (masters & workers), you must use a CoreOS version 962.0.0+ for the `kubelet-wrapper` script to be present in the image. If you wish to use an earlier version (e.g. from the 'stable' channel) see [kubelet-wrapper](kubelet-wrapper.md) for more information.

Use the official CoreOS bare metal guides for installation instructions:

* [Booting with iPXE][coreos-ipxe]
* [Booting with PXE][coreos-pxe]
* [Installing to Disk][coreos-ondisk]

Mixing multiple methods is possible. For example, doing an install to disk for the machines running the etcd cluster and Kubernetes master nodes, but PXE-booting the worker machines.

[coreos-ipxe]: https://coreos.com/os/docs/latest/booting-with-ipxe.html
[coreos-pxe]: https://coreos.com/os/docs/latest/booting-with-pxe.html
[coreos-ondisk]: https://coreos.com/os/docs/latest/installing-to-disk.html

### Kubernetes Pod Network

The following guides assume the use of [flannel][coreos-flannel] as a software-defined overlay network to manage routing of the [pod network][pod-network].
However, bare metal is a common platform where a self-managed network is used, due to the flexbility provided by physical networking gear.

See the [Kubernetes networking](kubernetes-networking.md) documentation for more information on self-managed networking options.

[coreos-flannel]: https://coreos.com/flannel/docs/latest/flannel-config.html
[pod-network]: https://github.com/kubernetes/kubernetes/blob/release-1.2/docs/design/networking.md#pod-to-pod

<div class="co-m-docs-next-step">
  <p><strong>Did you install CoreOS on your machines?</strong> An SSH connection to each machine is all that's needed. We'll start the configuration next.</p>
  <a href="getting-started.md" class="btn btn-primary btn-icon-right"  data-category="Getting Started" data-event="Getting Started">I'm ready to get started</a>
</div>

