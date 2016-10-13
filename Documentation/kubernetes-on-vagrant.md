# Kubernetes Installation with Vagrant &amp; CoreOS

This guide walks a deployer through launching a multi-node Kubernetes cluster using Vagrant and CoreOS.
After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the kubectl CLI tool.

## Install Prerequisites

### Vagrant

Navigate to the [Vagrant downloads page][vagrant-downloads] and grab the appropriate package for your system. Install the Vagrant software before continuing.

[vagrant-downloads]: https://www.vagrantup.com/downloads.html

### kubectl

`kubectl` is the main program for interacting with the Kubernetes API. Download `kubectl` from the Kubernetes release artifact site with the `curl` tool.

The linux `kubectl` binary can be fetched with a command like:

```sh
$ curl -O https://storage.googleapis.com/kubernetes-release/release/v1.4.1/bin/linux/amd64/kubectl
```

On an OS X workstation, replace `linux` in the URL above with `darwin`:

```sh
$ curl -O https://storage.googleapis.com/kubernetes-release/release/v1.4.1/bin/darwin/amd64/kubectl
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

The default cluster configuration is to start a virtual machine for each role &mdash; master node, worker node, and etcd server. However, you can modify the default cluster settings by copying `config.rb.sample` to `config.rb` and modifying configuration values.

```
#$update_channel="alpha"

#$controller_count=1
#$controller_vm_memory=512

#$worker_count=1
#$worker_vm_memory=512

#$etcd_count=1
#$etcd_vm_memory=512
```

By default, Calico network policy is disabled. To enable it, change the line `export USE_CALICO=false` to `export USE_CALICO=true` in both the `../generic/controller-install.sh` and the `../generic/worker-install.sh` scripts.

Also by default, the container runtime used is docker. To use rkt as the container runtime, change the line `export CONTAINER_RUNTIME=docker` to `export CONTAINER_RUNTIME=rkt` in both the `../generic/controller-install.sh` and the `../generic/worker-install.sh` scripts.

Ensure the latest CoreOS vagrant image will be used by running `vagrant box update`.

Then run `vagrant up` and wait for Vagrant to provision and boot the virtual machines.

## Configure kubectl

Choose one of the two following ways to configure `kubectl` to connect to the new cluster:

### Use a custom KUBECONFIG path

```sh
$ export KUBECONFIG="${KUBECONFIG}:$(pwd)/kubeconfig"
$ kubectl config use-context vagrant-multi
```

### Update the local-user kubeconfig

Configure your local Kubernetes client using the following commands:

```sh
$ kubectl config set-cluster vagrant-multi-cluster --server=https://172.17.4.101:443 --certificate-authority=${PWD}/ssl/ca.pem
$ kubectl config set-credentials vagrant-multi-admin --certificate-authority=${PWD}/ssl/ca.pem --client-key=${PWD}/ssl/admin-key.pem --client-certificate=${PWD}/ssl/admin.pem
$ kubectl config set-context vagrant-multi --cluster=vagrant-multi-cluster --user=vagrant-multi-admin
$ kubectl config use-context vagrant-multi
```

Check that `kubectl` is configured properly by inspecting the cluster:

```sh
$ kubectl get nodes
NAME          LABELS                               STATUS
172.17.4.201   kubernetes.io/hostname=172.17.4.201   Ready
```

**NOTE:** When the cluster is first launched, it must download all container images for the cluster components (Kubernetes, dns, heapster, etc). Depending on the speed of your connection, it can take a few minutes before the Kubernetes api-server is available. Before the api-server is running, the kubectl command above may show output similar to:

`The connection to the server 172.17.4.101:443 was refused - did you specify the right host or port?`

<div class="co-m-docs-next-step">
  <p><strong>Is kubectl working correctly?</strong></p>
  <p>Now that you've got a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.
Start with a <a href="https://github.com/kubernetes/kubernetes/blob/release-1.4/examples/guestbook/README.md" data-category="Docs Next" data-event="kubernetes.io: Guestbook">multi-tier web application</a> from the official Kubernetes documentation to visualize how the various Kubernetes components fit together.</p>
  <a href="https://github.com/kubernetes/kubernetes/blob/release-1.4/examples/guestbook/README.md" class="btn btn-default btn-icon-right" data-category="Docs Next" data-event="kubernetes.io: Guestbook">View the Guestbook example app</a>
</div>
