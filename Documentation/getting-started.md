# CoreOS &#43; Kubernetes Step By Step

This guide will walk you through a deployment of a single-master/multi-worker Kubernetes cluster on CoreOS. We're going to configure or deploy:

- an etcd cluster for Kubernetes to use
- generate the required certificates for communication between Kubernetes components
- deploy our Master node
- deploy our Worker nodes
- configure `kubectl` to work with our cluster
- deploy the DNS add-on

Working through this guide may take you a few hours, but it will give you good understanding of the moving pieces of your cluster and set you up for success in the long run. Let's get started.

## Deployment Options

The following variables will be used throughout this guide. Most of the provided defaults can safely be used, however some values such as `ETCD_ENDPOINTS` and `MASTER_IP` will need to be customized to your infrastructure.

**MASTER_IP**=_no default_

The IP address of the master node. Worker nodes must be able to reach the master via this IP on port 443. Additionally, external clients (such as an administrator using `kubectl`) will also need access, since this will run the Kubernetes API endpoint.

<hr/>

**ETCD_ENDPOINTS**=_no default_

List of etcd machines (`http://ip:port`), comma separated. If you're running a cluster of 5 machines, list them all here.

<hr/>

**POD_NETWORK**=10.2.0.0/16

The CIDR network to use for pod IPs.
Each pod launched in the cluster will be assigned an IP out of this range.
This network must be routable between all nodes in the cluster. In a default installation, the flannel overlay network will provide routing to this network.

<hr/>

**SERVICE_IP_RANGE**=10.3.0.0/24

The CIDR network to use for service cluster IPs. Each service will be assigned a cluster IP out of this range. This must not overlap with any IP ranges assigned to the POD_NETWORK, or other existing network infrastructure. Routing to these IPs is handled by a kube-proxy service local to each node, and are not required to be routable between nodes.

<hr/>

**K8S_SERVICE_IP**=10.3.0.1

The IP address of the Kubernetes API Service. If the SERVICE_IP_RANGE is changed above, this must be set to the first IP in that range.

<hr/>

**DNS_SERVICE_IP**=10.3.0.10

The IP address of the cluster DNS service. This IP must be in the range of the SERVICE_IP_RANGE and cannot be the first IP in the range. This same IP must be configured on all worker nodes to enable DNS service discovery.

## Deploy etcd Cluster

Kubernetes uses etcd for data storage and for cluster consensus between different software components. Your etcd cluster will be heavily utilized since all objects storing within and every scheduling decision is recorded. It's recommended that you run a multi-machine cluster on dedicated hardware (with fast disks) to gain maximum performance and reliability of this important part of your cluster. For development environments, a single etcd is ok.

### Single-Node (Development)

You can simply start etcd via [cloud-config][cloud-config-etcd] when you create your CoreOS machine or start it manually:

```
$ sudo systemctl start etcd2
```

To ensure etcd starts after a reboot, enable it too:

```sh
$ sudo systemctl enable etcd2
```

Record the IP address of an network interface on this machine that is reachable from your Kubernetes master, which will be configured below. In the rest of this guide, use that IP in the form `http://$IP:2379` as the `ETCD_ENDPOINTS`.

[cloud-config-etcd]: https://coreos.com/os/docs/latest/cloud-config.html#etcd2

### Multi-Node (Production)

It is highly recommended that etcd is run as a dedicated cluster separately from Kubernetes components.

Use the [official etcd clustering guide](https://coreos.com/etcd/docs/latest/clustering.html) to decide how best to deploy etcd into your environment.

## Generate Kubernetes TLS Assets

The Kubernetes API has various methods for validating clients &mdash; this guide will configure the API server to use client cert authentication.

This means it is necessary to have a Certificate Authority and generate the proper credentials. This can be done by generating the necessary assets from existing PKI infrastructure, or follow the OpenSSL instructions to create everything needed.

[OpenSSL Manual Generation](openssl.md)

In the following steps, it is assumed that you will have generated the following TLS assets:

**Root CA Public Key**

ca.pem

<hr/>

**API Server Public & Private Keys**

apiserver.pem

apiserver-key.pem

<hr/>

**Worker Node Public & Private Keys**

worker.pem

worker-key.pem

<hr/>

**Cluster Admin Public & Private Keys**

admin.pem

admin-key.pem

<div class="co-m-docs-next-step">
  <p><strong>Is your etcd cluster up and running?</strong> You need the IPs for the next step.</p>
  <p><strong>Did you generate all of the certificates</strong> You will place these on disk next.</p>
  <a href="deploy-master-single.md" class="btn btn-primary btn-icon-right">Yes, ready to deploy the Master</div>
</div>