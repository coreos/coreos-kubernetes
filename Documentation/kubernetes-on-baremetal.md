# Kubernetes Installation on Bare Metal &amp; CoreOS

This guide walks a deployer through launching a multi-node Kubernetes cluster on bare metal servers running CoreOS. After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the `kubectl` CLI tool.

## Deployment requirements

### CoreOS version

All Kubernetes controllers and nodes must use CoreOS version 962.0.0 or greater for the `kubelet-wrapper` script to be present in the image. If you wish to use an earlier version (e.g. from the 'stable' channel) see [kubelet-wrapper](kubelet-wrapper.md) for more information.

### Kubernetes pod network

This configuration uses the [flannel][coreos-flannel] overlay network to manage the [pod network][pod-network]. Many bare metal configurations may instead have an existing self-managed network. In this scenario, it is common to use [Calico][calico-networking] to manage pod network policy while omitting the overlay network, and interoperating with existing physical network gear over BGP.

See the [Kubernetes networking](kubernetes-networking.md) documentation for more information on self-managed networking options.

[coreos-flannel]: https://coreos.com/flannel/docs/latest/flannel-config.html
[calico-networking]: https://github.com/projectcalico/calico-containers
[pod-network]: https://github.com/kubernetes/kubernetes/blob/release-1.4/docs/design/networking.md#pod-to-pod

## Automated provisioning

Network booting and provisioning CoreOS clusters can be automated using the [coreos-baremetal](https://github.com/coreos/coreos-baremetal) project. It includes:

* Guides for configuring an network boot environment with iPXE/GRUB
* An HTTP/gRPC [service](https://github.com/coreos/coreos-baremetal/blob/master/Documentation/bootcfg.md) for booting and provisioning machines. Match machines by their hardware attributes and serve templated [Ignition](https://coreos.com/ignition/docs/latest/c) configs or cloud-configs.
* Example clusters including an [etcd cluster](https://github.com/coreos/coreos-baremetal/blob/master/Documentation/getting-started-rkt.md), multi-node [Kubernetes cluster](https://github.com/coreos/coreos-baremetal/blob/master/Documentation/kubernetes.md), and [self-hosted](https://github.com/coreos/coreos-baremetal/blob/master/Documentation/bootkube.md) Kubernetes cluster.

[Get started](https://github.com/coreos/coreos-baremetal#bootcfg) provisioning your machines into CoreOS clusters.

## Manual provisioning

Install CoreOS using the bare metal installation instructions:

* [Booting with iPXE][coreos-ipxe]
* [Booting with PXE][coreos-pxe]
* [Installing to Disk][coreos-ondisk]

Mixing multiple methods is possible. For example, doing an install to disk for the machines running the etcd cluster and Kubernetes master nodes, but PXE-booting the worker machines.

[coreos-ipxe]: https://coreos.com/os/docs/latest/booting-with-ipxe.html
[coreos-pxe]: https://coreos.com/os/docs/latest/booting-with-pxe.html
[coreos-ondisk]: https://coreos.com/os/docs/latest/installing-to-disk.html

<div class="co-m-docs-next-step">
  <p><strong>Did you install CoreOS on your machines?</strong> An SSH connection to each machine is all that's needed. We'll start the configuration next.</p>
  <a href="getting-started.md" class="btn btn-primary btn-icon-right"  data-category="Getting Started" data-event="Getting Started">I'm ready to get started</a>
</div>

