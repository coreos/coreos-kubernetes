# Kubernetes Networking

<div class="k8s-on-tectonic">
<p class="k8s-on-tectonic-description">This repo is not in alignment with current versions of Kubernetes, and will not be active in the future. The CoreOS Kubernetes documentation has been moved to the <a href="https://github.com/coreos/tectonic-docs/tree/master/Documentation">tectonic-docs repo</a>, where it will be published and updated.</p>

<p class="k8s-on-tectonic-description">For tested, maintained, and production-ready Kubernetes instructions, see our <a href="https://coreos.com/tectonic/docs/latest/install/aws/index.html">Tectonic Installer documentation</a>. The Tectonic Installer provides a Terraform-based Kubernetes installation. It is open source, uses upstream Kubernetes and can be easily customized.</p>
</div>

## Network Model

The Kubernetes network model outlines three methods of component communication:

* Pod-to-Pod Communication
    * Each Pod in a Kubernetes cluster is assigned an IP in a flat shared networking namespace. This allows for a clean network model where Pods, from a networking perspective, can be treated much like VMs or physical hosts.

* Pod-to-Service Communication
    * Services are implemented by assigning Virtual IPs which clients can access and are transparently proxied to the Pods grouped by that service. Requests to the Service IPs are intercepted by a kube-proxy process running on all hosts, which is then responsible for routing to the correct Pod.

* External-to-Internal Communication
    * Accessing services from outside the cluster is generally implemented by configuring external loadbalancers which target all nodes in the cluster. Once traffic arrives at a node, it is routed to the correct Service backends via the kube-proxy.

See [Kubernetes Networking][kubernetes-network] for more detailed information on the Kubernetes network model and motivation.

[kubernetes-network]: https://kubernetes.io/docs/admin/networking/

## Port allocation

The information below describes a minimum set of port allocations used by Kubernetes components. Some of these allocations will be optional depending on the deployment (e.g. if flannel or Calico is being used). Additionally, there are likely additional ports a deployer will need to open on their infrastructure (e.g. 22/ssh).

Master Node Inbound

| Protocol | Port Range | Source                                    | Purpose                |
-----------|------------|-------------------------------------------|------------------------|
| TCP      | 443        | Worker Nodes, API Requests, and End-Users | Kubernetes API server. |
| UDP      | 8285       | Master & Worker Nodes                   | flannel overlay network - *udp backend*. This is the default network configuration (only required if using flannel) |
| UDP      | 8472       | Master & Worker Nodes                   | flannel overlay network - *vxlan backend* (only required if using flannel) |

Worker Node Inbound

| Protocol | Port Range  | Source                         | Purpose                                                                |
-----------|-------------|--------------------------------|------------------------------------------------------------------------|
| TCP      | 10250       | Master Nodes                   | Worker node Kubelet API for exec and logs.                                  |
| TCP      | 10255       | Heapster                       | Worker node read-only Kubelet API.                                  |
| TCP      | 30000-32767 | External Application Consumers | Default port range for [external service][external-service] ports. Typically, these ports would need to be exposed to external load-balancers, or other external consumers of the application itself. |
| TCP      | ALL         | Master & Worker Nodes          | Intra-cluster communication (unnecessary if `vxlan` is used for networking)           |
| UDP      | 8285        | Master & Worker Nodes                   | flannel overlay network - *udp backend*. This is the default network configuration (only required if using flannel) |
| UDP      | 8472        | Master & Worker Nodes                   | flannel overlay network - *vxlan backend* (only required if using flannel) |
| TCP      | 179         | Worker Nodes                   | Calico BGP network (only required if the BGP backend is used) |

etcd Node Inbound

| Protocol | Port Range | Source        | Purpose                                                  |
-----------|------------|---------------|----------------------------------------------------------|
| TCP      | 2379-2380  | Master Nodes  | etcd server client API                                   |
| TCP      | 2379-2380  | Worker Nodes  | etcd server client API (only required if using flannel or Calico). |

[external-service]: http://kubernetes.io/docs/user-guide/services/#publishing-services---service-types

## Advanced Configuration

The CoreOS Kubernetes documentation describes a software-defined overlay network (i.e. [flannel][coreos-flannel]) to manage the Kubernetes Pod Network. However, in some cases a deployer may want to make use of existing network infrastructure to manage the Kubernetes network themselves e.g. using [Calico][calico]

The following requirements must be met by your existing infrastructure to use Tectonic with a self-managed network.

[coreos-flannel]: https://coreos.com/flannel/docs/latest/flannel-config.html
[calico]: http://docs.projectcalico.org/v2.0/getting-started/kubernetes/

### Pod-to-Pod Communication

Each pod in the Kubernetes cluster will be assigned an IP that is expected to be routable from all other hosts and pods in the Kubernetes cluster.

An easy way to achieve this is to use Calico. The Calico agent is already running on each node to enforce network policy. Starting it with the `CALICO_NETWORKING` environment variable set to `true` will cause it to run a BGP agent inside the Calico agent pod. These BGP agents will automatically form a full mesh network to exchange routing information. This allows a single large IP range to be used across your whole cluster and IP addresses to be efficiently assigned from it.  To peer with your existing BGP infrastructure follow this [guide][calico-bgp]. If your Kubernetes cluster is hosted on an [L2 network][calico-l2] (e.g. in your own datacenter or on AWS) there is no need to peer with your routers.

An alternative way to achieve this is to first assign an IP range to each host in your cluster.
Requests to IPs in an assigned range would need to be routed to that host via your network infrastructure.
Next, the host is configured such that each pod launched on the host is assigned an IP from the host range.

For example:

* Node A assigned IP range 10.0.1.0/24
* Node B assigned IP range 10.0.2.0/24.

When a Pod is launched on `Node A` it might be assigned `10.0.1.33` and on `Node B` a pod could be assigned `10.0.2.144`.
It would then be expected that both pods would be able to reach each other via those IPs, as if they were on a flat network.

The actual allocation of Pod IPs on the host can be achieved by configuring Docker to use a linux bridge device configured with the correct IP range.
 When a new Kubernetes Pod is launched, it will be assigned an IP from the range assigned to the linux bridge device.

To achieve this network model, there are various methods that can be used. See the [Kubernetes Networking][how-to-achieve] documentation for more detail.

[how-to-achieve]: https://kubernetes.io/docs/admin/networking/#how-to-achieve-this
[calico-bgp]: https://github.com/projectcalico/calico-containers/blob/v0.19.0/docs/bgp.md
[calico-l2]: http://docs.projectcalico.org/v2.0/reference/private-cloud/l2-interconnect-fabric

### Pod-to-Service Communication

The service IPs are assigned from a range configured in the Kubernetes API Server via the `--service-ip-range` flag. These are virtual IPs which are intercepted by a kube-proxy process running locally on each node. These IPs do not need to be routable off-host, because IPTables rules will intercept the traffic, and route to the proper backend (usually the pod network).

A requirement of a manually configured network is that the service-ip-range does not conflict with existing network infrastructure. The CoreOS Kubernetes guides default to a service-ip-range of `10.3.0.0/24`, but that can easily be changed if this conflicts with existing infrastructure.

### External-to-Internal Communication

IP addresses assigned on the pod network are typically not routable outside of the cluster unless you're using Calico and have [peered with your routers][calico-external]. This isn't an issue since most communication between your applications stays within the cluster, as described above. Allowing external traffic into the cluster is generally accomplished by mapping external load-balancers to specifically exposed services in the cluster. This mapping allows the kube-proxy process to route the external requests to the proper pods using the cluster's pod-network.

In a manually configured network, it may be necessary to open a range of ports to outside clients (default 30000-32767) for use with "external services". See the [Kubernetes Service][kube-service] documentation for more information on external services.

[calico-external]: https://github.com/projectcalico/calico-containers/blob/v0.19.0/docs/ExternalConnectivity.md
[kube-service]: http://kubernetes.io/docs/user-guide/services/#publishing-services---service-types
