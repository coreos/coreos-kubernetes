package config

const DefaultClusterConfig = `
# Unique name of Kubernetes cluster. In order to deploy
# more than one cluster into the same AWS account, this
# name must not conflict with an existing cluster.

clusterName: {{.ClusterName}}

# DNS name routable to the Kubernetes controller nodes
# from worker nodes and external clients. The deployer
# is responsible for making this name routable
externalDNSName: {{.ExternalDNSName}}

### DO NOT CHANGE PARAMETERS ABOVE HERE ###

# Name of the SSH keypair already loaded into the AWS
# account being used to deploy this cluster.
keyName: {{.KeyName}}

# Region to provision Kubernetes cluster
region: {{.Region}}

# Availability Zone to provision Kubernetes cluster
#availabilityZone:

# Instance type for controller node
#controllerInstanceType: m3.medium

# Disk size (GiB) for controller node
#controllerRootVolumeSize: 30

# Number of worker nodes to create
#workerCount: 1

# Instance type for worker nodes
#workerInstanceType: m3.medium

# Disk size (GiB) for worker nodes
#workerRootVolumeSize: 30

# Price (Dollars) to bid for spot instances. Omit for on-demand instances.
# workerSpotPrice: "0.05"

# CIDR for Kubernetes VPC
# vpcCIDR: "10.0.0.0/16"

# CIDR for Kubernetes subnet
# instanceCIDR: "10.0.0.0/24"

# IP Address for controller in Kubernetes subnet
# controllerIP: 10.0.0.50

# CIDR for all service IP addresses
# serviceCIDR: "10.3.0.0/24"

# CIDR for all pod IP addresses
# podCIDR: "10.2.0.0/16"

# IP address of Kubernetes controller service (must be contained by serviceCIDR)
# kubernetesServiceIP: 10.3.0.1

# IP address of Kubernetes dns service (must be contained by serviceCIDR)
# dnsServiceIP: 10.3.0.10

# Make a the controller available on a public IP address
# controllerPublic: "true"

# ID of the VPC to create the resources in. If provided together with subnetID,
# no network new resources will be created.
# vpcID: "vpc-xxxxxx"

# ID of the subnet to create the resources in. If provided together with vpcID,
# no network new resources will be created. 
# Make sure availabilityZone is set to availability zone of the subnet you specified
# Make sure controllerIP is set to an IP addres from the subnet range
# subnetID: "subnet-xxxxxx"
`
