# AWS Network Setup

These instructions assume the default network configuration used in the sample [controller-cloud-config](generic/controller-cloud-config.yaml) and [worker-cloud-config](generic/worker-cloud-config.yaml) files.

The following actions are all taken through the AWS Web Console.

## Deploy a VPC

Using the VPC wizard, follow the prompts for creation of a "VPC with a Single Public Subnet", allocating the network 10.0.0.0/16 and creating a subnet with the network 10.0.0.0/24.

After creating the VPC, enable auto-assignment of public IPs for the public subnet:

1. Navigate to the VPC service
2. Click "Subnets" on the left side navigation bar
3. Select the public subnet associated with your VPC
4. Click "Modify Auto-Assign Public IP" at the top of the page
5. Ensure the checkbox next to "Enable auto-assign Public IP" is checked and click "Save"

## Create Security Group

Create a single security group for the VPC created in the previous step.
After creation, assign the following inbound rules ($self refers to the ID of the security group):

| Type            | Protocol | Port Range  | Source    | Purpose |
|-----------------|----------|-------------|-----------|---------|
| Custom TCP Rule | TCP      | 22          | 0.0.0.0/0 | (optional) Allow SSH access to the cluster from all external IPs. |
| Custom TCP Rule | TCP      | 443         | $self     | Kubernetes API listens on port 443 by default, and how worker nodes contact the API. |
| Custom TCP Rule | TCP      | 2379-2380   | $self     | These are the two ports needed for etcd, assuming etcd is deployed in-cluster. If etcd is deployed off-cluster, the nodes will only need access to etcd via port 2379. |
| Custom TCP Rule | TCP      | 10248-10252 | $self     | Kubernetes uses these ports for some of its internal health checks. |
| Custom TCP Rule | TCP      | 30000-32767 | 0.0.0.0/0 | Kubernetes can optionally expose services in this port range. If Kubernetes services will not ever be exposed outside of the cluster, this rule is not necessary. |
| Custom UDP Rule | UDP      | 8285        | $self     | flannel routes betweek worker nodes using this port. |

Ensure a single outbound rule exists, permitting all traffic:

| Type            | Protocol | Port Range | Destination |
|-----------------|----------|------------|-------------|
| ALL Traffic     | ALL      | ALL        | 0.0.0.0/0   |

