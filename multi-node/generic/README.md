# WARNING: THIS IS A WIP & DOES NOT FULLY DOCUMENT A FUNCTIONAL INSTALLATION

# Multi Node CoreOS + Kubernetes

These instructions will walk you through deploying a multi-node Kubernetes cluster on CoreOS

## Kubernetes Deployment

### Prerequisites

#### etcd

It is the responsibility of the deployer to provide access to etcd to all Kubernetes nodes.

It is suggested that etcd is run off-cluster.
Use the [official etcd clustering guide](https://coreos.com/etcd/docs/latest/clustering.html) to decide how best to deploy etcd into your environment.

### Step 1: Prepare Controller cloud-config

It is suggested that a deployer start from the sample [controller cloud-config](controller-cloud-config.yaml) provided alongside this guide.

* Replace the `{{ETCD_ENDPOINTS}}` with comma separated list of etcd servers (http://ip:port)
* The remaining configurable items are documented in the sample, and can be left as their defaults if IP ranges do not conflict with any existing network infrastructure.

### Step 2: Prepare Worker Configuration

It is suggested that a deployer start from the sample [worker cloud-config](worker-cloud-config.yaml) provided alongside this guide.

* Replace the `{{ETCD_ENDPOINTS}}` with comma separated list of etcd servers (http://ip:port)
* Replace `{{CONTROLLER_ENDPOINT}}` with the endpoint where the controller nodes can be contacted (https://ip:port). In HA configurations this will typically be an external DNS record, or loadbalancer in front of the cluster control nodes.
* If the `DNS_SERVICE_IP` was modified from the default when deploying control nodes, the same value must be used in the worker cloud-config.

### Step 3: Prepare Network

Once the node configuration has been prepared, follow an infrastructure-specific network guide below:

* [Amazon Web Services](aws-network.md)
* [Generic Infrastructure](generic-network.md)

### Step 4: Deploy nodes

You will need to deploy a minimum of 1 controller and 1 worker. Depending on your deployment environment follow one of the sections below.

#### Amazon Web Services

* The [CoreOS EC2 documentation](https://coreos.com/os/docs/latest/booting-on-ec2.html) can be used to find the latest CoreOS AMI IDs and instructions on how to use the cloud-config's from the previous steps.

#### Generic Infrastructure

* If deploying onto baremetal servers, the [CoreOS Install Utility](https://coreos.com/os/docs/latest/installing-to-disk.html) may be useful to assist in deployment.

### Step 5: Distribute Keys to Nodes

TODO

### Step 6: Configure kubectl

Configure your local Kubernetes client using the following commands:

```
kubectl config set-cluster vagrant --server=https://{{CONTROLLER_ENDPOINT}}:443 --certificate-authority=${PWD}/ssl/ca.pem
kubectl config set-credentials vagrant-admin --certificate-authority=${PWD}/ssl/ca.pem --client-key=${PWD}/ssl/admin-key.pem --client-certificate=${PWD}/ssl/ad
kubectl config set-context vagrant --cluster=vagrant --user=vagrant-admin
kubectl config use-context vagrant
```

Check that your client is configured properly by using `kubectl` to inspect your cluster:

```
% kubectl get nodes
NAME          LABELS                               STATUS
x.x.x.x       kubernetes.io/hostname=x.x.x.x       Ready
```

### Step 7: Deploy an Application

Now that you've got a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.
Start with a [multi-tier web application][guestbook] from the official Kubernetes documentation to visualize how the various Kubernetes components fit together

[guestbook]: http://kubernetes.io/v1.0/examples/guestbook-go/README.html

