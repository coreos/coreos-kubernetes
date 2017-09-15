# Inspect a Kubernetes worker node

Master nodes and worker nodes differ in only a few ways. Most importantly, worker nodes don’t have public IP addresses.

To connect to a worker node, this example will use a master node as a jump box and SSH agent forwarding to connect.

First, confirm that there is an [SSH agent][ssh-agent] set up and running. Then connect to a master, then a worker from that master:

```
$ ssh -A core@<master>
Container Linux by CoreOS
$ ssh core@<worker>
```

Inspect the kubelet service to spot the differences:

```
$ systemctl cat kubelet
```

As you can see, only the node label is different, but the rest of the configuration is exactly the same. This cluster configuration defines multiple nodes which act as replicas of one another. These nodes are replaceable and interchangeable. Creating worker nodes in this manner allows you to use Kubernetes APIs to control the nodes, rather than having to directly manipulate config files on disk. This also allows you to take advantage of integral Kubernetes features, like RBAC and audit logging, without incurring additional developer debt.


[ssh-agent]: https://developer.github.com/v3/guides/using-ssh-agent-forwarding/
