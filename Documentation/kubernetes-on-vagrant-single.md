# Single-Node Kubernetes Installation with Vagrant &amp; CoreOS

While Kubernetes is designed to run across large clusters, it can be useful to have Kubernetes available on a single machine.
This guide walks a deployer through this process using Vagrant and CoreOS.
After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the kubectl CLI tool.

## Install Prerequisites

### Vagrant

Navigate to the [Vagrant downloads page][vagrant-downloads] and grab the appropriate package   for your system. Install the downloaded software before continuing.

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
$ cd coreos-kubernetes/single-node/
```

## Choose Container Runtime (optional)

The runtime defaults to docker. If you wish to use rkt simply edit the user-data and change the line beginning with `export CONTAINER_RUNTIME` to:

`export CONTAINER_RUNTIME=rkt`

## Start the Machine

Ensure the latest CoreOS vagrant image will be used by running `vagrant box update`.

Simply run `vagrant up` and wait for the command to succeed.
Once Vagrant is finished booting and provisioning your machine, your cluster is good to go.

## Configure kubectl

Once in the `coreos-kubernetes/single-node/` directory, configure your local Kubernetes client using the following commands:

You can choose from one of the two following options.

1. **Use a custom KUBECONFIG path**

   ```sh
   $ export KUBECONFIG="${KUBECONFIG}:$(pwd)/kubeconfig"
   $ kubectl config use-context vagrant-single
   ```

1. **Update the local-user kubeconfig**

   ```sh
   $ kubectl config set-cluster vagrant-single-cluster --server=https://172.17.4.99:443 --certificate-authority=${PWD}/ssl/ca.pem
   $ kubectl config set-credentials vagrant-single-admin --certificate-authority=${PWD}/ssl/ca.pem --client-key=${PWD}/ssl/admin-key.pem --client-certificate=${PWD}/ssl/admin.pem
   $ kubectl config set-context vagrant-single --cluster=vagrant-single-cluster --user=vagrant-single-admin
   $ kubectl config use-context vagrant-single
   ```

Check that your client is configured properly by using `kubectl` to inspect your cluster:

```sh
$ kubectl get nodes
NAME          LABELS                               STATUS
172.17.4.99   kubernetes.io/hostname=172.17.4.99   Ready
```

**NOTE:** When the cluster is first being launched, it must download all container images for the cluster components (Kubernetes, dns, heapster, etc). Depending on the speed of your connection, it can take a few minutes before the Kubernetes api-server is available. Before the api-server is running, the kubectl command above may show output similar to:

`The connection to the server 172.17.4.99:443 was refused - did you specify the right host or port?`

<div class="co-m-docs-next-step">
  <p><strong>Is kubectl working correctly?</strong></p>
  <p>Now that you've got a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.
Start with a <a href="https://github.com/kubernetes/kubernetes/blob/release-1.4/examples/guestbook/README.md" data-category="Docs Next" data-event="kubernetes.io: Guestbook">multi-tier web application</a> from the official Kubernetes documentation to visualize how the various Kubernetes components fit together.</p>
  <a href="https://github.com/kubernetes/kubernetes/blob/release-1.4/examples/guestbook/README.md" class="btn btn-default btn-icon-right" data-category="Docs Next" data-event="kubernetes.io: Guestbook">View the Guestbook example app</a>
</div>
