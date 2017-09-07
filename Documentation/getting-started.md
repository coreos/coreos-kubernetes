# CoreOS &#43; Kubernetes Part by Part

This guide will use Tectonic Installer to deploy a Kubernetes cluster on Amazon AWS, then part it out, break the system, and watch it rebuild automatically.

These instructions require an AWS account. See [Creating an AWS account][creating-aws] for more information.

## Cluster overview

Use Tectonic Installer to deploy Kubernetes clusters. Built on top of 100% open-source Kubernetes, Tectonic offers best practices solutions to cluster design and administration, including the following features:

* Highly Available Kubernetes: Easily deploy multiple masters and workers of Kubernetes.
Governance and User Management: Govern with RBAC and integrate with your existing identity infrastructure: LDAP, SAML.
* Secure Networking: Set up secure networking policies with Flannel and Calico.
* Highly Available etcd with Disaster Recovery: Deploy HA etcd with built-in backup and restore capabilities.
* Automated Operations: Easily update and maintain your infrastructure with CoreOS’s Automated Operations.
* Container Linux Operating System: Leverage a lightweight Linux distribution, built for containers.
* Prometheus Monitoring and AlertManager: Monitor your cluster and applications with Prometheus. Create alerts using Tectonic Console.
* Logging & Auditing: Audit your infrastructure, tracking all API generated actions.

## Deploy cluster

First, use Tectonic Installer to deploy a cluster on AWS using a [GUI interface][aws-installer], or through the command line using [Terraform][aws-terraform].

Then, follow these guides to analyze, break, and watch your cluster rebuild.
* [Set up kubectl][configure-kubectl]
* [Inspect the control plane][deploy-master]
* [Inspect a Kubernetes worker node][deploy-workers]
* [Break system locally and watch recovery][watch-recovery]


[creating-aws]: https://coreos.com/tectonic/docs/latest/tutorials/creating-aws.html
[aws-terraform]: https://coreos.com/tectonic/docs/latest/install/aws/aws-terraform.html
[aws-installer]: https://coreos.com/tectonic/docs/latest/install/aws/index.html
[configure-kubectl]: configure-kubectl.md
[deploy-master]: deploy-master.md
[deploy-workers]: deploy-workers.md
[watch-recovery]: watch-recovery.md
