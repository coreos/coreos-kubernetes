# Kubernetes on CoreOS with Generic Install Scripts

This guide will setup Kubernetes on CoreOS in a similar way to other tools in the repo. The main goal of these scripts is to be generic and work on many different cloud providers or platforms. The notable difference is that these scripts are intended to be platform agnostic and thus don't automatically setup the TLS assets on each host beforehand.

While we provide these scripts and test them through the multi-node Vagrant setup, we recommend using a platform specific install method if available. If you are installing to bare-metal, you might find our [baremetal repo](https://github.com/coreos/coreos-baremetal) more appropriate.

## Generate TLS Assets

Review the [OpenSSL-based TLS instructions][openssl] for generating your TLS assets for each of the Kubernetes nodes.

Place the files in the following locations:

| Controller Files | Location |
|------------------|----------|
| API Certificate | `/etc/kubernetes/ssl/apiserver.pem` |
| API Private Key | `/etc/kubernetes/ssl/apiserver-key.pem` |
| CA Certificate | `/etc/kubernetes/ssl/ca.pem` |

| Worker Files | Location |
|------------------|----------|
| Worker Certificate | `/etc/kubernetes/ssl/worker.pem` |
| Worker Private Key | `/etc/kubernetes/ssl/worker-key.pem` |
| CA Certificate | `/etc/kubernetes/ssl/ca.pem` |

## Network Requirements

This cluster must adhere to the [Kubernetes networking model][networking]. Nodes created by the generic scripts, by default, listen on and identify themselves by the `ADVERTISE_IP` environment variable. If this isn't set, the scripts will source it from `/etc/environment`, specifically using the value of `COREOS_PUBLIC_IPV4`.

### Controller Requirements

Each controller node must set its `ADVERTISE_IP` to an IP that accepts connections on port 443 from the workers. If using a load balancer, it must accept connections on 443 and pass that to the pool of controllers.

To view the complete list of environment variables, view the top of the `controller-install.sh` script.

### Worker Requirements

In addition to identifying itself with `ADVERTISE_IP`, each worker must be configured with the `CONTROLLER_ENDPOINT` variable, which tells them where to contact the Kubernetes API. For a single controller, this is the `ADVERTISE_IP` mentioned above. For multiple controllers, this is the IP of the load balancer.

To view the complete list of environment variables, view the top of the `worker-install.sh` script.

## Optional Configuration

You may modify the kubelet's unit file to use [additional features][rkt-opts-examples] such as:

- [mounting ephemeral disks][mount-disks]
- [allow pods to mount RDB][rdb] or [iSCSI volumes][iscsi]
- [allowing access to insecure container registries][insecure-registry]
- [use host DNS configuration instead of a public DNS server][host-dns]
- [enable the cluster logging add-on][cluster-logging]
- [changing your CoreOS auto-update settings][update]

## Boot etcd Cluster

It is highly recommended that etcd is run as a dedicated cluster separately from Kubernetes components.

Use the [official etcd clustering guide](https://coreos.com/etcd/docs/latest/docker_guide.html) to decide how best to deploy etcd into your environment.

## Boot Controllers

Follow these instructions for each controller you wish to boot:

1. Boot CoreOS
1. [Download][controller-script] and copy `controller-install.sh` onto disk.
1. Copy TLS assets onto disk.
1. Execute `controller-install.sh` with environment variables set.
1. Wait for the script to complete. About 300 MB of containers will be downloaded before the cluster is running.

## Boot Workers

Follow these instructions for each worker you wish to boot:

1. Boot CoreOS
1. [Download][worker-script] and copy `worker-install.sh` onto disk.
1. Copy TLS assets onto disk.
1. Execute `worker-install.sh` with environment variables set.
1. Wait for the script to complete. About 300 MB of containers will be downloaded before the cluster is running.

## Monitor Progress

The Kubernetes will be up and running after the scripts complete and containers are downloaded. To take a closer look, SSH to one of the machines and monitor the container downloads:

```
$ docker ps
```

You can also watch the kubelet's logs with journalctl:

```
$ journalctl -u kubelet -f
```

<div class="co-m-docs-next-step">
  <p><strong>Did your containers start downloading?</strong> Next, set up the `kubectl` CLI for use with your cluster.</p>
  <a href="configure-kubectl.md" class="btn btn-primary btn-icon-right"  data-category="Docs Next" data-event="Kubernetes: kubectl">Yes, ready to configure `kubectl`</a>
</div>

[openssl]: openssl.md
[networking]: kubernetes-networking.md
[rkt-opts-examples]: kubelet-wrapper.md#customizing-rkt-options
[rdb]: kubelet-wrapper.md#allow-pods-to-use-rbd-volumes
[iscsi]: kubelet-wrapper.md#allow-pods-to-use-iscsi-mounts
[host-dns]: kubelet-wrapper.md#use-the-hosts-dns-configuration
[cluster-logging]: kubelet-wrapper.md#use-the-cluster-logging-add-on
[mount-disks]: https://coreos.com/os/docs/latest/mounting-storage.html
[insecure-registry]: https://coreos.com/os/docs/latest/registry-authentication.html#using-a-registry-without-ssl-configured
[update]: https://coreos.com/os/docs/latest/switching-channels.html
[controller-script]: ../multi-node/generic/controller-install.sh
[worker-script]: ../multi-node/generic/worker-install.sh
