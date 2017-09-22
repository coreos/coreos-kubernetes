# Kubernetes Installation on Bare Metal &amp; CoreOS

<div class="k8s-on-tectonic">
<p class="k8s-on-tectonic-description">This repo is not in alignment with current versions of Kubernetes, and will not be active in the future. The CoreOS Kubernetes documentation has been moved to the [tectonic-docs repo](https://github.com/coreos/tectonic-docs/tree/master/Documentation), where it will be published and updated.</p>

<p class="k8s-on-tectonic-description">For tested, maintained, and production-ready Kubernetes instructions, see our [Tectonic Installer documentation](https://coreos.com/tectonic/docs/latest/install/aws/index.html). The Tectonic Installer provides a Terraform-based Kubernetes installation. It is open source, uses upstream Kubernetes and can be easily customized.</p>
</div>

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

The CoreOS [Matchbox][matchbox-gh] project can automate network booting and provisioning Container Linux clusters. It provides:

* The Matchbox HTTP/gRPC service matches machines to configs, by hardware attributes, and can be installed as a binary, RPM, container image, or deployed on Kubernetes itself.
* Guides for creating network boot environments with iPXE/GRUB
* Support for Terraform to allow teams to manage and version bare-metal resources
* Example clusters including an [etcd cluster][etcd-cluster-example] and multi-node [Kubernetes cluster][kubernetes-cluster-example].

[Get started][matchbox-intro-doc] provisioning machines into clusters or read the [docs][matchbox-docs].

Container Linux bare metal installation documents provide low level background details about the boot mechanisms:

* [Booting with iPXE][coreos-ipxe]
* [Booting with PXE][coreos-pxe]
* [Installing to Disk][coreos-ondisk]

Mixing multiple methods is possible. For example, doing an install to disk for the machines running the etcd cluster and Kubernetes master nodes, but PXE-booting the worker machines.

[coreos-ipxe]: https://coreos.com/os/docs/latest/booting-with-ipxe.html
[coreos-pxe]: https://coreos.com/os/docs/latest/booting-with-pxe.html
[coreos-ondisk]: https://coreos.com/os/docs/latest/installing-to-disk.html
[ignition-docs]: https://coreos.com/ignition/docs/latest/
[matchbox-gh]: https://github.com/coreos/matchbox
[matchbox-docs]: https://coreos.com/matchbox/docs/latest/
[matchbox-intro-doc]: https://coreos.com/matchbox/docs/latest/getting-started.html
[etcd-cluster-example]: https://github.com/coreos/matchbox/blob/master/Documentation/getting-started-rkt.md
[kubernetes-cluster-example]: https://coreos.com/matchbox/docs/latest/terraform/bootkube-install/README.html

<div class="co-m-docs-next-step">
  <p><strong>Did you install CoreOS on your machines?</strong> An SSH connection to each machine is all that's needed. We'll start the configuration next.</p>
  <a href="getting-started.md" class="btn btn-primary btn-icon-right"  data-category="Getting Started" data-event="Getting Started">I'm ready to get started</a>
</div>
