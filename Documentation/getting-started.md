# CoreOS &#43; Kubernetes Part by Part

This guide will use Tectonic Installer to deploy a Kubernetes cluster on Amazon AWS, then part it out, deconstruct the system, and break/fix the system in various places.

These instructions require an AWS account. See [Creating an AWS account][creating-aws] for more information.

## Cluster overview

Give folks an idea of what we are deploying
* Use programmatic bring up via terraform
** Repeatable
** Automatable
** Customizable
* Nodes look very similar, just a few labels different (more on this below)
** Little state on disk means it can be more dynamic using Kubernetes API
* Use a self-hosted cluster to keep the “smarts” in the cluster (more on this below)

## Deploy cluster using terraform

Follow the instructions: https://coreos.com/tectonic/docs/latest/install/aws/aws-terraform.html


[creating-aws]: https://coreos.com/tectonic/docs/latest/tutorials/creating-aws.html
[tectonic-installer]: https://github.com/coreos/tectonic-installer
