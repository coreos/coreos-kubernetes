# Kubernetes on CoreOS with Generic Install Scripts

<div class="k8s-on-tectonic">
<p class="k8s-on-tectonic-description">This repo is not in alignment with current versions of Kubernetes, and will not be active in the future. The CoreOS Kubernetes documentation has been moved to the <a href="https://github.com/coreos/tectonic-docs/tree/master/Documentation">tectonic-docs repo</a>, where it will be published and updated.</p>

<p class="k8s-on-tectonic-description">For tested, maintained, and production-ready Kubernetes instructions, see our <a href="https://coreos.com/tectonic/docs/latest/install/aws/index.html">Tectonic Installer documentation</a>. The Tectonic Installer provides a Terraform-based Kubernetes installation. It is open source, uses upstream Kubernetes and can be easily customized.</p>
</div>

This guide will setup Kubernetes on CoreOS in a similar way to other tools in the repo. The main goal of these scripts is to be generic and work on many different cloud providers or platforms. The notable difference is that these scripts are intended to be platform agnostic and thus don't automatically setup the TLS assets on each host beforehand.

[Read the documentation to boot a cluster][docs]

[docs]: ../../Documentation/kubernetes-on-generic-platforms.md
