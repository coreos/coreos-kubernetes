# Inspect the control plane

Once the cluster is fully booted, use kubectl or Tectonic Console to inspect the assets started by the bootstrap process.

In the Console, click the Pods section and select *Namespace: kube-system* from the pulldown menu at the top of the page. This will show you all the pods that make up the Kubernetes control plane.

<div class="row">
  <div class="col-lg-8 col-lg-offset-2 col-md-10 col-md-offset-1 col-sm-12 col-xs-12">
    <img src="/img/PodNamespaceMenu.png">
    <div class="co-m-screenshot-caption">Namespace pulldown menu</div>
  </div>
</div>

Or, use kubectl to list all pods in the namespace:

```
$ kubectl --namespace=kube-system get deployments
```

Both techniques will display a number of components running on the cluster. Most of these components use only a small amount of available resources. The number and complexity of the components spun up demonstrate how easy it is to deploy a cluster using Kubernetes.

At the cluster level, two types of Kubernetes objects, Deployments and DaemonSets allow us to scale these components across machines for higher availability. These two objects function very closely and use the same underlying concept, a [Pod][pod].

**Deployments** are used to run multiple copies of a Pod _anywhere_ in the cluster. The cluster will decide where these can best run based on available resources and other factors.

**DaemonSets** are used to run copies of a Pod on _every_ node, or on a subset of nodes that match a label query. As the number of matched nodes changes, the number of Pods stay in sync.

Deployments exist for components like the Kubernetes Scheduler, Controller Manager, and DNS server to enable them to run in a highly available fashion _anywhere_ in the cluster. Use the Deployments page in Tectonic Console or `kubectl --namespace=kube-system get deployments` to review your Deployments.

DaemonSets exist for the Kubernetes proxy and flannel to ensure that they run on every node. Kubernetes masters and workers differ in their Kubelet flags, most notably in the `--node-labels` flag. These flags will be used in conjunction with a “node selector” to run the API server on nodes labeled `node-role.kubernetes.io/master`. Since the API server is critical to the cluster, this allows for easy scale out and simplifies networking, as the master's autoscaling group can be placed directly behind a load balancer. The address of the load balancer was shown earlier when you ran `kubectl cluster-info`.

Both the Kubernetes proxy and flannel objects build off the Pod, which is why you see so many Pods running in the namespace. **Reconciliation loops**  are utilized by both objects to ensure the correct Pods are running at all times.

## Inspect the deployed node locally

With the cluster up and kubectl working, explore the cluster to see its components.

First, use kubectl to list the Kubernetes's Node Resources; Nodes are the name Kubernetes gives any machine or virtual machine in a Kubernetes cluster.

```
$ kubectl get nodes
NAME                                        STATUS    AGE
ip-10-0-19-110.us-west-2.compute.internal   Ready     2d
ip-10-0-2-146.us-west-2.compute.internal    Ready     2d
ip-10-0-36-204.us-west-2.compute.internal   Ready     2d
ip-10-0-77-101.us-west-2.compute.internal   Ready     2d
```

This command returns the list of running nodes, with internal AWS hostnames.

Use `kubectl get nodes -o wide` to create a table displaying internal and external IPs for every node in the cluster. Use the ‘external’ column to find external IPs.

```
$ kubectl get nodes -o wide
i-0e475be181f7e1973 10.0.19.110 52.35.157.146
i-03d1bc5e86b7e8741 10.0.2.146 34.209.11.147
i-038ea56416abdd140 10.0.36.204 34.210.154.200
i-024d0aa51334a068b 10.0.77.101
```

By default only the master Nodes, the machines running the Kubernetes API server, will have external IPs. Choose one of the public IPs from the third column and login.

```
ssh -A core@34.209.11.147
```

The important piece of software running is the Kubernetes machine agent, called the kubelet. The  kubelet talks to the Kubernetes API server to:
* receive tasks to start/stop
* report back node health, resource usage and metadata
* Stream back real-time data like logs from applications

```
$ systemctl cat kubelet
```

A few configuration flags determine important parts of the configuration:

|Flag   	|Description   	|
|---	|---	|
|``--kubeconfig`   	|A path to a kubeconfig file on disk. This is the same format that was configured above for kubectl, but with different permissions. This is placed on disk by the Tectonic installer.   	|
|``--node-labels`   	|A piece of metadata about the machine, which is useful for customizing where workloads run. For example, this node is labeled with `node-role.kubernetes.io/master` in order to run special master workloads on it.   	|
|``--client-ca-file`   	|Another credential that was placed on disk by the Tectonic installer.   	|
|``--cloud-provider`   	|Provides hooks into a cloud provider, for creating load balancers or disk automatically.   	|

Inspect the other assets placed on disk:

```
$ ls /etc/kubernetes/
```

The kubelet systemd unit and the files placed on disk by the Tectonic Installer make up the bulk of our machine customization. This reflects the goal to have the “smarts” live in the cluster, and have each node be dumb and replaceable. It is simple to add new capacity, or replace a failed node.

Later, we will intentionally break the kubelet on both a master and a worker to explore failure scenarios.

[pod]: https://coreos.com/kubernetes/docs/latest/pods.html
