# Break system locally and watch recovery

One of the core features of Kubernetes is that it’s designed to maintain the desired state defined by operators and app admins.

The API server acts as the brains of the cluster and is run as a Deployment with multiple Pods to ensure High Availability. Each Deployment runs on a master node.

First, make sure you are running a sample app. Follow the [Tectonic Sandbox tutorials][first-app] to launch a simple app, or use one of your own. With a running app, we will first intentionally kill a copy of the API server, and then simulate a node failure. After each of these, we will watch as the cluster recovers.

## Simulate Pod failure

First, let’s demonstrate that the Control Plane is run in High Availability by intentionally killing an API server Pod process, and watching it recover. This will also show the rescheduling process of Replication Controllers and reconciliation loops.

Open a terminal and a browser window with Tectonic Console. Keep them open side by side for this exercise. Kubernetes will rebuild the killed containers quickly.

First, use Tectonic Console to determine which nodes are running the API server.
Go to *Workloads > Pods*, and enter ‘api’ in the search field.

Then, select one of the nodes listed, and SSH to its IP address.

```
$ ssh core@ip
```

Remember that the kubelet runs on **reconciliation loops** which work to keep the system in a stable state. When we execute the command to kill this API server, the kubelet will notice immediately.

Test this by killing the active containers running the Kubernetes API server on this node. Because the API server is a DaemonSet, it should be running on all of the master nodes.

From one of the masters, get the IDs of the containers running the API Server:

```
$ docker ps -f 'name=.*kube-apiserver.*' --format "{{.ID}}"
4ae686927f22
06bb5ba95034
```
Then, kill the listed containers.

```
$ docker kill 4ae686927f22 06bb5ba95034
```

Within a few seconds the kubelet will notice that the API server Pod is no longer running, and will relaunch under a new ID:

```
$ docker ps -f 'name=.*kube-apiserver.*' --format "{{.ID}}"
f4a262619d1d
d964fa94c69b
```

In the console, watch the list of API servers to see that one is disabled, and a new one is launched. Note that the Tectonic Console session offers continuous feedback because the API is configured with High Availability. This is the same Kubernetes feature that enables your apps to remain up and running without constant developer monitoring and interaction.

## Simulate Node failure

Killing the API server acts as an example of an individual process failure. Next, simulate a power down or network partition by disabling an entire node.

Kubelet uses reconciliation loops to remain in constant contact with the API server and report back status. Use iptables to create a temporary firewall rule and simulate an interruption in this connection.

```
$ iptables -A OUTPUT -p tcp --dport 443  -j DROP
$ systemctl stop dockerk
$ systemctl stop kubelet
$ ps aux | grep kube
```

Use Tectonic Console to inspect the node. Notice that the node is marked ‘unhealthy’, and nothing is rescheduled.

Check your app. Even though the node is unhealthy, and the rescheduling process has not yet begun, the app is still up and running as before. Because the cluster runs in High Availability, the app does not go down, and your clients experience no interruption in service. Kubernetes maintains persistent availability automatically.

Wait 5 minutes, then reinspect Console. Notice that new nodes have been scheduled to repair the interruption.

Tectonic configures the kubelet to wait 5 minutes after a node is determined to be unhealthy before workloads are moved off the node. Because nodes are run in High Availability, redundancy allows the system time to regroup.

Undo the temporary firewall rule to watch the node recover almost immediately.

```
$ iptables -A OUTPUT -p tcp --dport 443  -j ACCEPT
$ systemctl start dockerk
$ systemctl enable kubelet
$ ps aux | grep kube

```


[first-app]: https://coreos.com/tectonic/docs/latest/tutorials/sandbox/first-app.html
