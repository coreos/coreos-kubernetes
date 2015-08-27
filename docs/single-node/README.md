# Single Node CoreOS + Kubernetes

These instructions will walk you through deploying a single-node Kubernetes stack on CoreOS.

CoreOS-vagrant will be used in this example, but these steps can be used to deploy on a CoreOS system in any environment using the included cloud-config.

NOTE: A single-node stack is not recommended for production workloads

## Deploy on CoreOS Vagrant

### Step 1: Clone CoreOS Vagrant repo

```
git clone https://github.com/coreos/coreos-vagrant.git
cd coreos-vagrant
```

### Step 2: Create the cloud-config

Copy the provided single-node [cloud-config](../../cloud-config/single-node-cloud-config.yaml) into the `user-data` file in the coreos-vagrant directory.

### Step 3: Start the Vagrant node

```
vagrant up
```

### Step 4: Connect & inspect state

```
vagrant ssh core-01
```

It will take a few minutes to download all of the assets. You can watch the status of bootstrap:

```
journalctl -fu bootstrap
```

*NOTE*: Bootstrap is complete when you see: `systemd[1]: Started bootstrap.service`

## Next Steps

### Query Kubernetes API

Once the Kubernetes API is running, you can use the `kubectl` tool to query for running pods:

```
core@core-01 ~ $ kubectl get pods --all-namespaces
NAMESPACE     NAME                                   READY     STATUS    RESTARTS   AGE
kube-system   kube-apiserver-172.17.8.101            1/1       Running   0          2m
kube-system   kube-controller-manager-172.17.8.101   1/1       Running   0          2m
kube-system   kube-dns-v8-eyfuz                      4/4       Running   0          2m
kube-system   kube-podmaster-172.17.8.101            2/2       Running   0          2m
kube-system   kube-scheduler-172.17.8.101            1/1       Running   0          2m
```

### Deploy Sample Application

Simple multi-tier web application: [Guestbook Example](http://kubernetes.io/v1.0/examples/guestbook-go/README.html)
