# Multi Node CoreOS + Kubernetes

These instructions will walk you through deploying a multi-node Kubernetes cluster on CoreOS

## Kubernetes Deployment

### Prerequisites

#### etcd

It is the responsibility of the deployer to provide access to etcd to all Kubernetes nodes.

It is suggested that etcd is run off-cluster.
Use the [official etcd clustering guide](https://coreos.com/etcd/docs/latest/clustering.html) to decide how best to deploy etcd into your environment.

### Step 1: Prepare Controller cloud-config

It is suggested that a deployer start from the sample [controller cloud-config](generic/controller-cloud-config.yaml) provided alongside this guide.

* Replace the `{{ETCD_ENDPOINTS}}` with comma separated list of etcd servers (http://ip:port)
* The remaining configurable items are documented in the sample, and can be left as their defaults if IP ranges do not conflict with any existing network infrastructure.

### Step 2: Prepare Worker Configuration

It is suggested that a deployer start from the sample [worker cloud-config](generic/worker-cloud-config.yaml) provided alongside this guide.

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

### Step 5: Distribute Keys to Controller

In order for the kube-controller-manager to create tokens for service accounts, it needs access to an RSA private key.
Generate this key using a command like the following:

```
openssl genrsa -out service-account-private-key.pem 4096
```

Distribute the same key securely to each controller node, placing it at `/etc/kubernetes/service-account-private-key.pem`.

### Step 6: Inspect State

It will take a few minutes for the bootstrap process to download all of the assets. You can watch the status of bootstrap process on controller & worker nodes, by ssh'ing into the machine and querying the systemd journal:

```
journalctl -fu bootstrap
```

*NOTE*: Bootstrap is complete when you see: `systemd[1]: Started bootstrap.service`

Once the controller and worker nodes are running, you can use the `kubectl` tool to query for running pods:

```
core@core-01 ~ $ kubectl get pods --all-namespaces
NAMESPACE     NAME                                   READY     STATUS    RESTARTS   AGE
kube-system   kube-apiserver-172.17.8.101            1/1       Running   0          2m
kube-system   kube-apiserver-172.17.8.102            1/1       Running   0          2m
kube-system   kube-controller-manager-172.17.8.102   1/1       Running   0          2m
kube-system   kube-dns-v8-eyfuz                      4/4       Running   0          2m
kube-system   kube-podmaster-172.17.8.101            2/2       Running   0          2m
kube-system   kube-podmaster-172.17.8.102            2/2       Running   0          2m
kube-system   kube-scheduler-172.17.8.101            1/1       Running   0          2m
```

## Next Steps

### Deploy Sample Application

Simple multi-tier web application: [Guestbook Example](http://kubernetes.io/v1.0/examples/guestbook-go/README.html)

