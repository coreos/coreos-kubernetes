# Generic Network Setup

These instructions assume the default network configuration used in the sample [controller-cloud-config](generic/controller-cloud-config.yaml) and [worker-cloud-config](worker-cloud-config.yaml) files.

## Network Configuration

| Type            | Protocol | Port Range  | Source    | Purpose |
|-----------------|----------|-------------|-----------|---------|
| Custom TCP Rule | TCP      | 22          | 0.0.0.0/0 | (optional) Allow SSH access to the cluster from all external IPs. |
| Custom TCP Rule | TCP      | 443         | $self     | Kubernetes API listens on port 443 by default, and how worker nodes contact the API. |
| Custom TCP Rule | TCP      | 2379-2380   | $self     | These are the two ports needed for etcd, assuming etcd is deployed in-cluster. If etcd is deployed off-cluster, the nodes will only need access to etcd via port 2379. |
| Custom TCP Rule | TCP      | 10248-10252 | $self     | Kubernetes uses these ports for some of its internal health checks. |
| Custom TCP Rule | TCP      | 30000-32767 | 0.0.0.0/0 | Kubernetes can optionally expose services in this port range. If Kubernetes services will not ever be exposed outside of the cluster, this rule is not necessary. |
| Custom UDP Rule | UDP      | 8285        | $self     | flannel routes betweek worker nodes using this port. |

All traffic must be permitted to leave the cluster.
