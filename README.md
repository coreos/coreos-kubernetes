# Kubernetes on CoreOS Container Linux

<div class="k8s-on-tectonic">
<p class="k8s-on-tectonic-description">This repo is not in alignment with current versions of Kubernetes, and will not be active in the future. The CoreOS Kubernetes documentation has been moved to the <a href="https://github.com/coreos/tectonic-docs/tree/master/Documentation">tectonic-docs repo</a>, where it will be published and updated.</p>

<p class="k8s-on-tectonic-description">For tested, maintained, and production-ready Kubernetes instructions, see our <a href="https://coreos.com/tectonic/docs/latest/install/aws/index.html">Tectonic Installer documentation</a>. The Tectonic Installer provides a Terraform-based Kubernetes installation. It is open source, uses upstream Kubernetes and can be easily customized.</p>
</div>

This repo contains tooling and documentation around deploying Kubernetes using CoreOS Container Linux.
Initial setup of a Kubernetes cluster is covered, but ongoing maintenance and updates of the cluster is not addressed.

*Notice: kube-aws has moved!*

If you're looking for kube-aws, it has been moved to a new [dedicated repository](https://github.com/coreos/kube-aws). All outstanding AWS-related issues and PRs should be moved to there. This repository will continue to host development on single and multi node vagrant distributions.

## The CoreOS Way

When designing these guides and tools, the following considerations are made:

* We always setup TLS
* An individual node can reboot and the cluster will still function
* Internal cluster DNS is available
* Service accounts enabled
* Follow Kubernetes guidelines for AdmissionControllers and other suggested configuration

## Kubernetes Topics

Follow the Kubernetes guides on the CoreOS website:

https://coreos.com/kubernetes/docs/latest/

 - [Intro to Pods](https://coreos.com/kubernetes/docs/latest/pods.html)
 - [Intro to Services](https://coreos.com/kubernetes/docs/latest/services.html)
 - [Intro to Replication Controllers](https://coreos.com/kubernetes/docs/latest/replication-controller.html)

## Deploying on Container Linux

- [Step-by-Step for Any Platform](Documentation/getting-started.md)
- [Single-Node Vagrant Stack](single-node/README.md)
- [Multi-Node Vagrant Cluster](multi-node/vagrant/README.md)
- [Multi-Node Bare Metal Cluster](Documentation/kubernetes-on-baremetal.md)

## Running Kubernetes Conformance Tests

- [Conformance Tests](Documentation/conformance-tests.md)
