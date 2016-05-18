# Kubernetes on AWS

Deploy a fully-functional Kubernetes cluster using AWS CloudFormation.
Your cluster will be configured to use AWS features to enhance Kubernetes.
For example, Kubernetes may automatically provision an Elastic Load Balancer for each Kubernetes Service.
After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the kubectl CLI tool.

At CoreOS, we use the [kube-aws](https://github.com/coreos/coreos-kubernetes/releases) CLI tool to automate cluster deployment to AWS.

## Download pre-built binary

Import the [CoreOS Application Signing Public Key](https://coreos.com/security/app-signing-key/):

```sh
gpg2 --keyserver pgp.mit.edu --recv-key FC8A365E
```

Validate the key fingerprint:

```sh
gpg2 --fingerprint FC8A365E
```
The correct key fingerprint is `18AD 5014 C99E F7E3 BA5F  6CE9 50BD D3E0 FC8A 365E`

Go to the [releases](https://github.com/coreos/coreos-kubernetes/releases) and download the latest release tarball and detached signature (.sig) for your architecture.

Validate the tarball's GPG signature:

```sh
PLATFORM=linux-amd64
# Or
PLATFORM=darwin-amd64

gpg2 --verify kube-aws-${PLATFORM}.tar.gz.sig kube-aws-${PLATFORM}.tar.gz
```
Extract the binary:

```sh
tar zxvf kube-aws-${PLATFORM}.tar.gz
```

Add kube-aws to your path:

```sh
mv ${PLATFORM}/kube-aws /usr/local/bin
```

## Configure AWS credentials

Configure your local workstation with AWS credentials using one of the following methods:

### Method 1: Environment variables

Set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` to the values of your AWS access and secret keys, respectively:

```sh
$ export AWS_ACCESS_KEY_ID=AKID1234567890
$ export AWS_SECRET_ACCESS_KEY=MY-SECRET-KEY
```

### Method 2: Config file

Write your credentials into the file `~/.aws/credentials` using the following template:

```
[default]
aws_access_key_id = AKID1234567890
aws_secret_access_key = MY-SECRET-KEY
```

## Configure cluster

First, let's define a few parameters that we'll use when we create the cluster.

### EC2 key pair

The keypair that will authenticate SSH access to your EC2 instances. More info in the [EC2 Keypair docs](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html).

### External DNS name

Select a DNS hostname where the cluster's API will be accessible. This information will first be used to provision the TLS certificate for the API server.

When CloudFormation finishes creating your cluster, your controller will expose the TLS-secured API via a public IP address. You will need to create an A record for the DNS hostname which lists the IP address of the API. You can find this IP address later via `kube-aws status`.

`kube-aws` can be optionally be configured to automatically create an A record in an existing route53 hosted zone.

### KMS key

[Amazon KMS](http://docs.aws.amazon.com/kms/latest/developerguide/overview.html) keys are used to encrypt and decrypt cluster TLS assets. If you already have a KMS Key that you would like to use, you can skip this step.

Creating a KMS key can be done via the [AWS web console](http://docs.aws.amazon.com/kms/latest/developerguide/create-keys.html) or via the AWS cli tool:

```sh
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

Reference the `KeyMetadata.Arn` string on the next step.

### Initialize an asset directory

Create a directory on your local machine that will hold the generated assets, then initialize your cluster:

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

There will now be a `cluster.yaml` file in the asset directory.

### Render contents of the asset directory

Next, generate a default set of cluster assets in your asset directory:

```sh
$ kube-aws render
```

These assets (templates and credentials) are used to create, update and interact with your Kubernetes cluster.

You can now customize your cluster by editing asset files:

* **cluster.yaml**

  This is the configuration file for your cluster. It contains the configuration parameters that are templated into your userdata and CloudFormation stack.

* **userdata/**

  * `cloud-config-worker`
  * `cloud-config-controller`

  This directory contains the [cloud-init](https://github.com/coreos/coreos-cloudinit) cloud-config userdata files. The CoreOS operating system supports automated provisioning via cloud-config files, which describe the various files, scripts and systemd actions necessary to produce a working cluster machine. These files are templated with your cluster configuration parameters and embedded into the CloudFormation stack template.

* **stack-template.json**

  This file describes the [AWS CloudFormation](https://aws.amazon.com/cloudformation/) stack which encompasses all the AWS resources associated with your cluster. This JSON document is temlated with configuration parameters, we well as the encoded userdata files.

* **credentials/**

  This directory contains the **unencrypted** TLS assets for your cluster, along with a pre-configured `kubeconfig` file which provides access to your cluster api via kubectl.

You can also now check the `my-cluster` asset directory into version control if you desire. The contents of this directory are your reproducible cluster assets. Please take care not to commit the `my-cluster/credentials` directory, as it contains your TLS secrets. If you're using git, the `credentials` directory will already be ignored for you.

### Optional Calico network policy

The cluster can be configured to use Calico to provide network policy.

Edit the `cluster.yaml` file:
```yaml
useCalico: true
kubernetesVersion: v1.2.4_coreos.cni.0
```
The hyperkube image version needs to contain the CNI binaries (these are tagged with `_cni`)

### Optional Route53 Host Record

`kube-aws` can optionally create an A record for the controller IP in an existing hosted zone.

Edit the `cluster.yaml` file:

```yaml
externalDNSName: my-cluster.staging.core-os.net
createRecordSet: true
hostedZone: staging.core-os.net
```

If `createRecordSet` is not set to true, the deployer will be responsible for making externalDNSName routable to the controller IP after the cluster is created.

### Validate cluster assets

The `validate` command check the validity of the cloud-config userdata files and the CloudFormation stack description:

```sh
$ kube-aws validate
```

### Create a cluster from asset directory

Now for the exciting part, create your cluster:

```sh
$ kube-aws up
```

**PRODUCTION NOTE**: the TLS keys and certificates generated by `kube-aws` should *not* be used to deploy a production Kubernetes cluster.
Each component certificate is only valid for 90 days, while the CA is valid for 365 days.
If deploying a production Kubernetes cluster, consider establishing PKI independently of this tool first.

### Configure DNS

If you configured Route 53 settings in your configuration above via `createRecordSet`, a host record has already been created for you.

Otherwise, navigate to the DNS registrar hosting the zone for the provided external DNS name. Ensure a single A record exists, routing the value of `externalDNSName` defined in `cluster.yaml` to the externally-accessible IP of the master node instance.

You may use `kube-aws status` to get this value after cluster creation, if necessary. This command can take a while.

### Access the cluster

A kubectl config file will be written to a `kubeconfig` file, which can be used to interact with your Kubernetes cluster like so:

```sh
$ kubectl --kubeconfig=kubeconfig get nodes
```

**NOTE**: It can take some time after `kube-aws up` completes before the cluster is available. When the cluster is first being launched, it must download all container images for the cluster components (Kubernetes, dns, heapster, etc). Depending on the speed of your connection, it can take a few minutes before the Kubernetes api-server is available. Before the api-server is running, the kubectl command above may show output similar to:

 `The connection to the server <MASTER>:443 was refused - did you specify the right host or port?`

### Export the cloudformation stack

```sh
$ kube-aws up --export
```

### Destroy

When you are done with your cluster, simply run `kube-aws destroy` and all cluster components will be destroyed.
If you created any Kubernetes Services of type `LoadBalancer`, you must delete these first, as the CloudFormation cannot be fully destroyed if any externally-managed resources still exist.

### Certificates and Keys

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
