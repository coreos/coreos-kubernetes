# Kubernetes on AWS

Deploy a fully-functional Kubernetes cluster using AWS CloudFormation.
Your cluster will be configured to use AWS features to enhance Kubernetes.
For example, Kubernetes may automatically provision an Elastic Load Balancer for each Kubernetes Service.
After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the kubectl CLI tool.

At CoreOS, we use the [kube-aws](https://github.com/coreos/coreos-kubernetes/tree/master/multi-node/aws) CLI tool to automate cluster deployment to AWS.

### AWS Credentials
The supported way to provide AWS credentials to kube-aws is by exporting the following environment variables:

```sh
export AWS_ACCESS_KEY_ID=AKID1234567890
export AWS_SECRET_ACCESS_KEY=MY-SECRET-KEY
```

### Download kube-aws

```sh
PLATFORM=linux-amd64
# Or
PLATFORM=darwin-amd64

wget https://coreos-kubernetes.s3.amazonaws.com/kube-aws/latest/${PLATFORM}/kube-aws
chmod +x kube-aws
# Add kube-aws binary to your PATH
```

### Configure AWS Credentials

Configure your local workstation with AWS credentials using one of the following methods:

#### Method 1: Environment Variables

Set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` to the values of your AWS access and secret keys, respectively:

```sh
$ export AWS_ACCESS_KEY_ID=AKID1234567890
$ export AWS_SECRET_ACCESS_KEY=MY-SECRET-KEY
```

#### Method 2: Config File

Write your credentials into the file `~/.aws/credentials` using the following template:

```
[default]
aws_access_key_id = AKID1234567890
aws_secret_access_key = MY-SECRET-KEY
```

### Configure Cluster

#### EC2 Key Pair

The keypair that will authenticate SSH access to your ec2 instances. [Docs](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html)

#### External DNS Name

Before configuring the cluster, we to define a DNS hostname at which the cluster's API will be accessible. This information will first be used to provision the TLS certificate for the API server.

When Cloudformation finishes creating your cluster, your controller will expose the TLS-secured API via a public IP address. You will need to create an A record for the DNS hostname which lists the IP address of the API. You can find this IP address later via `kube-aws status`.

#### KMS Key

[Amazon KMS](http://docs.aws.amazon.com/kms/latest/developerguide/overview.html) keys are used to encrypt and decrypt cluster TLS assets. If you already have a KMS Key that you would like to use, you can skip this step.

Creating a KMS key can be done via the [AWS web console](http://docs.aws.amazon.com/kms/latest/developerguide/create-keys.html) or via the AWS cli tool.

```shell
$ aws kms --region=<your-region> create-key --description="kube-aws assets"
{
    "KeyMetadata": {
        "CreationDate": 1458235139.724,
        "KeyState": "Enabled",
        "Arn": "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx",
        "AWSAccountId": "xxxxxxxxxxxxx",
        "Enabled": true,
        "KeyUsage": "ENCRYPT_DECRYPT",
        "KeyId": "xxxxxxxxx",
        "Description": "kube-aws assets"
    }
}
```
You'll need the `KeyMetadata.Arn` string for the next step.

#### Initialize an asset directory
```sh
$ mkdir my-cluster
$ cd my-cluster
$ kube-aws init --cluster-name=my-cluster-name \
--external-dns-name=my-cluster-endpoint \
--region=us-west-1 \
--availability-zone=us-west-1c \
--key-name=key-pair-name \
--kms-key-arn="arn:aws:kms:us-west-1:xxxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
```

There will now be a cluster.yaml file in the asset directory.

#### Render contents of the asset directory

```sh
$ kube-aws render
```
This generates the default set of cluster assets in your asset directory. These assets are templates and credentials that are used to create, update and interact with your Kubernetes cluster.

You can now customize your cluster by editing asset files:

* **cluster.yaml**

  This is the configuration file for your cluster. It contains the configuration parameters that are templated into your userdata and cloudformation stack.

* **cloud-config/**

  * `cloud-config-worker`
  * `cloud-config-controller`

  This directory contains the [cloud-init](https://github.com/coreos/coreos-cloudinit) cloud-config userdata files. The CoreOS operating system supports automated provisioning via cloud-config files, which describe the various files, scripts and systemd actions necessary to produce a working cluster machine. These files are templated with your cluster configuration parameters and embedded into the cloudformation stack template.

* **stack-template.json**

  This file describes the [AWS cloudformation](https://aws.amazon.com/cloudformation/) stack which encompasses all the AWS resources associated with your cluster. This JSON document is temlated with configuration parameters, we well as the encoded userdata files.

* **credentials/**

  This directory contains the **unencrypted** TLS assets for your cluster, along with a pre-configured `kubeconfig` file which provides access to your cluster api via kubectl.

You can also now check the `my-cluster` asset directory into version control if you desire. The contents of this directory are your reproducible cluster assets. Please take care not to commit the `my-cluster/credentials` directory, as it contains your TLS secrets. If you're using git, the `credentials` directory will already be ignored for you.

#### Validate your cluster assets

The `validate` command check the validity of the cloud-config userdata files and the cloudformation stack description.

```sh
$ kube-aws validate
```

#### Create a cluster from asset directory

```sh
$ kube-aws up
```

**PRODUCTION NOTE**: the TLS keys and certificates generated by `kube-aws` should *not* be used to deploy a production Kubernetes cluster.
Each component certificate is only valid for 90 days, while the CA is valid for 365 days.
If deploying a production Kubernetes cluster, consider establishing PKI independently of this tool first.

Navigate to the DNS registrar hosting the zone for the provided external DNS name and ensure a single A record exists, routing the value of `externalDNSName` defined in `cluster.yaml` to the externally-accessible IP of the master node instance.
You may use `kube-aws status` to get this value after cluster creation, if necessary.

This command can take a while.

#### Access the cluster

A kubectl config file will be written to a `kubeconfig` file, which can be used to interact with your Kubernetes cluster like so:

```sh
$ kubectl --kubeconfig=kubeconfig get nodes
```

It can take some time after `kube-aws up` completes before the cluster is available. Until then, you will have a `connection refused` error.

#### Export your cloudformation stack
```sh
$ kube-aws up --export
```

#### Destroy

When you are done with your cluster, simply run `kube-aws destroy` and all cluster components will be destroyed.
If you created any Kubernetes Services of type `LoadBalancer`, you must delete these first, as the CloudFormation cannot be fully destroyed if any externally-managed resources still exist.

#### Certs & Keys

`kube-aws render` begins by initializing the TLS infrastructure needed to securely operate Kubernetes. If you have your own key/certificate management system, you can overwrite the generated TLS assets after `kube-aws render`.

When `kube-aws up` creates the cluster stack, it will use whatever TLS assets it finds in the `credentials` folder at the time.

This includes the certificate authority, signed server certificates for the Kubernetes API server and workers, and a signed client certificate for administrative use.

* **APIServerCert, APIServerKey**

	The API server certificate will be valid for the value of `externalDNSName`, as well as a the DNS names used to route Kubernetes API requests inside the cluster.

	`kube-aws` does *not* manage a DNS zone for the cluster.
	This means that the deployer is responsible for ensuring the routability of the external DNS name to the public IP of the master node instance.

	The certificate and key granted to the kube-apiserver.
	This certificate will be presented to external clients of the Kubernetes cluster, so it should be valid for external DNS names, if necessary.

	Additionally, the certificate must have the following Subject Alternative Names (SANs).
	These IPs and DNS names are used within the cluster to route from applications to the Kubernetes API:

		- 10.0.0.50
		- 10.3.0.1
		- kubernetes
		- kubernetes.default
		- kubernetes.default.svc
		- kubernetes.default.svc.cluster.local

* **WorkerCert, WorkerKey**

	The certificate and key granted to the kubelets on worker instances.
	The certificate is shared across all workers, so it must be valid for all worker hostnames.
	This is achievable with the Subject Alternative Name (SAN) `*.*.cluster.internal`, or `*.ec2.internal` if using the us-east-1 AWS region.

* **CACert**

	The certificate authority's TLS certificate is used to sign other certificates in the cluster.

	These assets are stored unencrypted in your `credentials` folder, but are encyrpted using Amazon KMS before being embedded in the CloudFormation template.

	All keys and certs must be PEM-formatted and base64-encoded.
