# Kubernetes on CoreOS

This repo contains tooling and documentation around deploying Kubernetes using CoreOS.
Initial setup of a Kubernetes cluster is covered, but ongoing maintenance and updates of the cluster is not addressed.
It is officially maintained by the CoreOS team and meant to be a set of introductory documentation to get a feel for using Kubernetes on CoreOS.

## The CoreOS Way

When designing these guides and tools, the following considerations are made:

* We always setup TLS
* An individual node can reboot and the cluster will still function
* Internal cluster DNS is available
* Service accounts enabled
* Use a cloud-provider if we can, for instance on AWS
* Follow Kubernetes guidelines for AdmissionControllers and other suggested configuration

## Kubernetes Topics

Follow the Kubernetes guides on the CoreOS website:

https://coreos.com/kubernetes/docs/latest/

 - [Intro to Pods](https://coreos.com/kubernetes/docs/latest/pods.html)
 - [Intro to Services](https://coreos.com/kubernetes/docs/latest/services.html)
 - [Intro to Replication Controllers](https://coreos.com/kubernetes/docs/latest/replication-controller.html)

## Deploying on CoreOS

- [Step-by-Step for Any Platform](Documentation/getting-started.md)
- [Single-Node Vagrant Stack](single-node/README.md)
- [Multi-Node Vagrant Cluster](multi-node/vagrant/README.md)
- [Multi-Node AWS Cluster](multi-node/aws/README.md)
- [Multi-Node Bare Metal Cluster](Documentation/kubernetes-on-baremetal.md)

## Running Kubernetes Conformance Tests

- [Conformance Tests](Documentation/conformance-tests.md)
