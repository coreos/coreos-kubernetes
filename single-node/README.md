# Kubernetes on CoreOS Quickstart

While Kubernetes is designed to run across large clusters, it can be useful to have Kubernetes available on a single machine.
This guide walks a deployer through this process using Vagrant and CoreOS.
After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the kubectl CLI tool.

## Step 0: Install Prerequisites

### Vagrant

Navigate to the [Vagrant downloads page][vagrant-downloads] and grab the appropriate package   for your system. Install the downloaded software before continuing.

[vagrant-downloads]: https://www.vagrantup.com/downloads.html

## Step 1: Bring the cluster up

Simply run `vagrant up` and wait for the command to succeed.
Vagrant will boot and provision your machine but it will take a few minutes for the kubernetes component containers to be downloaded.
While the VM is working to get everything downloaded lets configure a local command line client, `kubectl`

## Step 2: Download kubectl

The primary CLI tool used to interact with the Kubernetes API is called `kubectl`.
This tool is not yet available through the typical means of software distribution, so it is suggested that you download the binary directly from the Kubernetes release artifact site:

First, download the binary using a command-line tool such as `wget` or `curl` from `https://storage.googleapis.com/kubernetes-release/release/v1.0.3/bin/${ARCH}/amd64/kubectl`.
Set the ARCH environment variable to "linux" or "darwin" based no your workstation operating system:

```
ARCH=linux wget https://storage.googleapis.com/kubernetes-release/release/v1.0.3/bin/$ARCH/amd64/kubectl
```

After downloading the binary, ensure it is executable and move it into your PATH:

```
chmod +x kubectl
mv kubectl /usr/local/bin/kubectl
```

Test that kubectl works by running the version command:

```
kubectl version
```


## Step 2: Configure kubectl


Now, lets configure your client to use the single node cluster we have started using the following commands:

```
kubectl config set-cluster vagrant --server=https://172.17.4.99:443 --certificate-authority=${PWD}/ssl/ca.pem
kubectl config set-credentials vagrant-admin --certificate-authority=${PWD}/ssl/ca.pem --client-key=${PWD}/ssl/admin-key.pem --client-certificate=${PWD}/ssl/admin.pem
kubectl config set-context vagrant --cluster=vagrant --user=vagrant-admin
kubectl config use-context vagrant
```

Check that your client is configured properly by using `kubectl` to inspect your single-node cluster:

```
% kubectl get nodes
NAME          LABELS                               STATUS
172.17.4.99   kubernetes.io/hostname=172.17.4.99   Ready
```

## Step 3: Deploy an Application

Now that you've got a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.
Start with a [multi-tier web application][guestbook] from the official Kubernetes documentation to visualize how the various Kubernetes components fit together.

[guestbook]: http://kubernetes.io/v1.0/examples/guestbook-go/README.html
