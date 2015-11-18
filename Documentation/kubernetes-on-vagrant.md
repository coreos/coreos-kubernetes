# Kubernetes Installation with Vagrant &amp; CoreOS

This guide walks a deployer through launching a multi-node Kubernetes cluster using Vagrant and CoreOS.
After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the kubectl CLI tool.

## Install Prerequisites

### Vagrant

Navigate to the [Vagrant downloads page][vagrant-downloads] and grab the appropriate package for your system. Install the downloaded software before continuing.

[vagrant-downloads]: https://www.vagrantup.com/downloads.html

### kubectl

The primary CLI tool used to interact with the Kubernetes API is called `kubectl`.
This tool is not yet available through the typical means of software distribution, so it is suggested that you download the binary directly from the Kubernetes release artifact site:

First, download the binary using a command-line tool such as `wget` or `curl` from `https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/${ARCH}/amd64/kubectl`.
Set the ARCH environment variable to "linux" or "darwin" based on your workstation operating system:

```sh
ARCH=linux; wget https://storage.googleapis.com/kubernetes-release/release/v1.1.1/bin/$ARCH/amd64/kubectl
```

After downloading the binary, ensure it is executable and move it into your PATH:

```sh
$ chmod +x kubectl
$ mv kubectl /usr/local/bin/kubectl
```

## Clone the Repository

The following commands will clone a repository that contains a "Vagrantfile", which describes the set of virtual machines that will run Kubernetes on top of CoreOS.

```sh
$ git clone https://github.com/coreos/coreos-kubernetes.git
$ cd coreos-kubernetes/multi-node/vagrant
```

## Start the Machines

The default cluster configuration is to start a virtual machine for each role &mdash; controller, worker, and etcd server. However, you can modify the default cluster settings by copying `config.rb.sample` to `config.rb` and modifying configuration values.

```
#$update_channel="alpha"

#$controller_count=1
#$controller_vm_memory=512

#$worker_count=1
#$worker_vm_memory=512

#$etcd_count=1
#$etcd_vm_memory=512
```

Next, simply run `vagrant up` and wait for the command to succeed.
Once Vagrant is finished booting and provisioning your machine, your cluster is good to go.

## Configure kubectl

Configure your local Kubernetes client using the following commands:

```sh
$ kubectl config set-cluster vagrant --server=https://172.17.4.101:443 --certificate-authority=${PWD}/ssl/ca.pem
$ kubectl config set-credentials vagrant-admin --certificate-authority=${PWD}/ssl/ca.pem --client-key=${PWD}/ssl/admin-key.pem --client-certificate=${PWD}/ssl/admin.pem
$ kubectl config set-context vagrant --cluster=vagrant --user=vagrant-admin
$ kubectl config use-context vagrant
```

Check that your client is configured properly by using `kubectl` to inspect your cluster:

```sh
$ kubectl get nodes
NAME          LABELS                               STATUS
172.17.4.201   kubernetes.io/hostname=172.17.4.201   Ready
```

<div class="co-m-docs-next-step">
  <p><strong>Is kubectl working correctly?</strong></p>
  <p>Now that you've got a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.
Start with a <a href="http://kubernetes.io/v1.1/examples/guestbook-go/README.html" data-category="Docs Next" data-event="kubernetes.io: Guestbook">multi-tier web application</a> from the official Kubernetes documentation to visualize how the various Kubernetes components fit together.</p>
  <a href="http://kubernetes.io/v1.1/examples/guestbook-go/README.html" class="btn btn-default btn-icon-right" data-category="Docs Next" data-event="kubernetes.io: Guestbook">View the Guestbook example app</a>
</div>
