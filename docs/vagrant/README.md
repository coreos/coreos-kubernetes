## Vagrant Cluster

The default cluster size is set to 1-controller, 1-worker, and 1-etcd server.

 However, you can modify the cluster settings by copying `cluster/vagrant/config.rb.sample` to `cluster/vagrant/config.rb` and modifying configuration values.

### Step 1: Launch Cluster

```
cd cluster/vagrant
vagrant up

# This will launch servers with hostnames:
# (controllers) = c1, c2, ...
# (workers) = w1, w2, ...
# (etcd) = e1, e2, ...
```

### Step 2: Inspect Controller State

*NOTE:* Bootstrap is complete when you see: `systemd[1]: Started bootstrap.service`

```
vagrant ssh c1
journalctl -lfu bootstrap
```

### Step 3: Query Kubernetes API

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

### Next Steps: Deploy Sample Application

Simple multi-tier web application: [Guestbook Example](http://kubernetes.io/v1.0/examples/guestbook-go/README.html)

## Teardown

```
vagrant destroy
```

