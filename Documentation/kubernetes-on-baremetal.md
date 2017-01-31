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

## Provisioning

Network booting and provisioning CoreOS clusters can be automated using the CoreOS [matchbox][matchbox-gh] project. It includes:

* Guides for creating network boot environments with iPXE/GRUB
* The matchbox HTTP/gRPC [service][matchbox-intro-doc] for booting and provisioning bare-metal machines. Match machines by their hardware attributes and serve templated [Ignition][ignition-docs] provisioning configurations.
* Example clusters including an [etcd cluster][etcd-cluster-example], multi-node [Kubernetes cluster][multi-node-example], and [self-hosted][self-hosted-example] Kubernetes cluster.

[Get started][matchbox-gh] provisioning your machines into CoreOS clusters.

The CoreOS bare metal installation documents provide background and deployment options for the boot mechanisms:

* [Booting with iPXE][coreos-ipxe]
* [Booting with PXE][coreos-pxe]
* [Installing to Disk][coreos-ondisk]

Mixing multiple methods is possible. For example, doing an install to disk for the machines running the etcd cluster and Kubernetes master nodes, but PXE-booting the worker machines.


[coreos-ipxe]: https://coreos.com/os/docs/latest/booting-with-ipxe.html
[coreos-pxe]: https://coreos.com/os/docs/latest/booting-with-pxe.html
[coreos-ondisk]: https://coreos.com/os/docs/latest/installing-to-disk.html
[etcd-cluster-example]: https://github.com/coreos/matchbox/blob/master/Documentation/getting-started-rkt.md
[ignition-docs]: https://coreos.com/ignition/docs/latest/
[matchbox-gh]: https://github.com/coreos/matchbox
[matchbox-intro-doc]: https://github.com/coreos/matchbox/blob/master/Documentation/matchbox.md
[multi-node-example]: https://github.com/coreos/matchbox/blob/master/Documentation/kubernetes.md
[self-hosted-example]: https://github.com/coreos/matchbox/blob/master/Documentation/bootkube.md

<div class="co-m-docs-next-step">
  <p><strong>Did you install CoreOS on your machines?</strong> An SSH connection to each machine is all that's needed. We'll start the configuration next.</p>
  <a href="getting-started.md" class="btn btn-primary btn-icon-right"  data-category="Getting Started" data-event="Getting Started">I'm ready to get started</a>
</div>
