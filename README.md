# Kubernetes on CoreOS

This repo contains a set of tools and documention for how to setup Kubernetes on CoreOS. It is officially maintained by the CoreOS team and meant to be a set of introducutory documentation to get a feel for using kubernetes on CoreOS. 

## "CoreOS way"

When designing these guides and tools, the following considerations are made: 

* We always setup TLS
* An individual node can reboot and the cluster will still function
* DNS addon is enabled
* Service accounts enabled
* Use the cloud-provider if we can, for instance on AWS
* Use 1.0 cluster docs for AdmissionControllers and other suggested guidelines

## Recommended guides

Kubernetes has many configurations, so there are different approaches based on your desired outcome. 

|Outcome|Recommended Guide|
|:--|:--|
|Quickly get local environment running|[All-in-one on Vagrant](#)|
|Quickly try kubernetes on AWS|[All-in-one on AWS](#)|
|HA control plane and multi worker-nodes on AWS|[Multi-node on AWS](#)|

## What is not covered

These guides and tools describe how to setup and operate a kubernetes cluster, but they do not provide tooling for on going maintenance and updating of the cluster. These guides assume that are you are starting from a fresh environment and want to get a feel for how to deploy kubernetes with various degrees of production readiness. 