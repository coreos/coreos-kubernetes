# Configure your Kubernetes cluster on AWS

This is the second step of [running Kubernetes on AWS][aws-step-1]. Before we launch our cluster, let's define a few parameters that the cluster requires.

## Cluster parameters

### EC2 key pair

The keypair that will authenticate SSH access to your EC2 instances. The public half of this key pair will be configured on each CoreOS node.

After creating a key pair, you will use the name you gave the keys to configure the cluster. Key pairs are only available to EC2 instances in the same region. More info in the [EC2 Keypair docs](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html).

### KMS key

[Amazon KMS](http://docs.aws.amazon.com/kms/latest/developerguide/overview.html) keys are used to encrypt and decrypt cluster TLS assets. If you already have a KMS Key that you would like to use, you can skip creating a new key and provide the Arn string for your existing key.

You can create a KMS key in the [AWS console](http://docs.aws.amazon.com/kms/latest/developerguide/create-keys.html), or with the `aws` command line tool:

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

You will use the KeyMetadata.Arn string to identify your KMS key in the init step.

### External DNS name

Select a DNS hostname where the cluster API will be accessible. Typically this hostname is available over the internet ("external"), so end users can connect from different networks. This hostname will be used to provision the TLS certificate for the API server, which encrypts traffic between your users and the API. Optionally, you can provide the certificates yourself, which is recommended for production clusters.

When the cluster is created, the controller will expose the TLS-secured API on a public IP address. You will need to create an A record for the external DNS hostname you want to point to this IP address. You can find the API external IP address after the cluster is created by invoking `kube-aws status`.

Alternatively, kube-aws can automatically create this A record in an *existing* [Route 53][route53] hosted zone. If you have a DNS zone hosted in Route 53, you can configure for it below.

## Initialize an asset directory

Create a directory on your local machine to hold the generated assets:

```sh
$ mkdir my-cluster
$ cd my-cluster
```

Initialize the cluster CloudFormation stack with the KMS Arn, key pair name, and DNS name from the previous step:

```sh
$ kube-aws init \
--cluster-name=my-cluster-name \
--external-dns-name=my-cluster-endpoint \
--region=us-west-1 \
--availability-zone=us-west-1c \
--key-name=key-pair-name \
--kms-key-arn="arn:aws:kms:us-west-1:xxxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
```

There will now be a `cluster.yaml` file in the asset directory. This is the main configuration file for your cluster.

### Render contents of the asset directory

* In the simplest case, you can have kube-aws generate both your TLS identities and certificate authority for you.

  ```sh
  $ kube-aws render --generate-credentials --generate-ca
  ```

  This is not recommended for production, but is fine for development or testing purposes.

* It is recommended that you supply your own immediate certificate signing authority and let kube-aws take care of generating the cluster TLS credentials.

  ```sh
  $ kube-aws render --generate-credentials --ca-cert-path=/path/to/ca-cert.pem --ca-key-path=/path/to/ca-key.pem
  ```

  For more information on operating your own CA, check out this [awesome guide](https://jamielinux.com/docs/openssl-certificate-authority/).

* In certain cases, such as users with advanced pre-existing PKI infrastructure, the operator may wish to pre-generate all cluster TLS assets. In this case, you can run `kube-aws render` and copy in your TLS assets into the `credentials/` folder before running `kube-aws up`.

Here's what the directory structure looks like:

```sh
$ tree
.
├── cluster.yaml
├── credentials
│   ├── admin-key.pem
│   ├── admin.pem
│   ├── apiserver-key.pem
│   ├── apiserver.pem
│   ├── ca-key.pem
│   ├── ca.pem
│   ├── worker-key.pem
│   └── worker.pem
│   ├── etcd-key.pem
│   └── etcd.pem
│   ├── etcd-client-key.pem
│   └── etcd-client.pem
├── kubeconfig
├── stack-template.json
└── userdata
    ├── cloud-config-controller
    └── cloud-config-worker
```

These assets (templates and credentials) are used to create, update and interact with your Kubernetes cluster.

At this point you should be ready to create your cluster. You can also check the `my-cluster` asset directory into version control if you desire. The contents of this directory are your reproducible cluster assets. Please take care not to commit the `my-cluster/credentials` directory, as it contains your TLS secrets. If you're using git, the `credentials` directory will already be ignored for you.

**PRODUCTION NOTE**: the TLS keys and certificates generated by `kube-aws` should *not* be used to deploy a production Kubernetes cluster.
Each component certificate is only valid for 90 days, while the CA is valid for 365 days.
If deploying a production Kubernetes cluster, consider establishing PKI independently of this tool first. [Read more below.][tls-note]

<div class="co-m-docs-next-step">
  <p><strong>Did everything render correctly?</strong></p>
  <p>If you are familiar with CoreOS and the AWS platform, you may want to include some additional customizations or optional features. Read on below to explore more.</p>
  <a href="kubernetes-on-aws-launch.md" class="btn btn-primary btn-icon-right"  data-category="Docs Next" data-event="Kubernetes: AWS Launch">Yes, ready to launch the cluster</a>
  <a href="kubernetes-on-aws-render.md#customizations-to-your-cluster" class="btn btn-link btn-icon-right"  data-category="Docs Next" data-event="Kubernetes: AWS Customizations">View optional features &amp; customizations</a>
</div>

## Customizations to your cluster

You can now customize your cluster by editing asset files. Any changes to these files will require a `render` and `validate` workflow, covered below.

### Customize infrastructure

* **cluster.yaml**

  This is the configuration file for your cluster. It contains the configuration parameters that are templated into your userdata and CloudFormation stack.

  Some common customizations are:

  - change the number of workers
  - specify tags applied to all created resources
  - create cluster inside an existing VPC
  - change controller and worker volume sizes
  <br/><br/>

* **userdata/**

  * `cloud-config-worker`
  * `cloud-config-controller`

  This directory contains the [cloud-init](https://github.com/coreos/coreos-cloudinit) cloud-config userdata files. The CoreOS operating system supports automated provisioning via cloud-config files, which describe the various files, scripts and systemd actions necessary to produce a working cluster machine. These files are templated with your cluster configuration parameters and embedded into the CloudFormation stack template.

  Some common customizations are:

  - [mounting ephemeral disks][mount-disks]
  - [allow pods to mount RDB][rdb] or [iSCSI volumes][iscsi]
  - [allowing access to insecure container registries][insecure-registry]
  - [use host DNS configuration instead of a public DNS server][host-dns]
  - [enable the cluster logging add-on][cluster-logging]
  - [changing your CoreOS auto-update settings][update]
  <br/><br/>

* **stack-template.json**

  This file describes the [AWS CloudFormation](https://aws.amazon.com/cloudformation/) stack which encompasses all the AWS resources associated with your cluster. This JSON document is templated with configuration parameters, we well as the encoded userdata files.

  Some common customizations are:

  - tweak AutoScaling rules and timing
  - instance IAM roles
  - customize security groups beyond the initial configuration
  <br/><br/>

* **credentials/**

  This directory contains the **unencrypted** TLS assets for your cluster, along with a pre-configured `kubeconfig` file which provides access to your cluster api via kubectl.

[mount-disks]: https://coreos.com/os/docs/latest/mounting-storage.html
[insecure-registry]: https://coreos.com/os/docs/latest/registry-authentication.html#using-a-registry-without-ssl-configured
[update]: https://coreos.com/os/docs/latest/cloud-config.html#update

### Kubernetes Container Runtime

The kube-aws tool now optionally supports using rkt as the kubernetes container runtime. To configure rkt as the container runtime you must run with a CoreOS version >= `v1151.0.0` and configure the runtime flag.

Edit the `cluster.yaml` file:

```yaml
containerRuntime: rkt
releaseChannel: alpha
```

Note that while using rkt as the runtime is now supported, it is still a new option as of the Kubernetes v1.3 release and has a few [known issues](http://kubernetes.io/docs/getting-started-guides/rkt/notes/).

### Calico network policy

The cluster can be optionally configured to use Calico to provide network policy. These policies limit and control how different pods, namespaces, etc can communicate with each other. These rules can be managed after the cluster is launched, but the feature needs to be turned on beforehand.

Edit the `cluster.yaml` file:

```yaml
useCalico: true
```

### Route53 Host Record

`kube-aws` can optionally create an A record for the controller IP in an existing route53 hosted zone.

Edit the `cluster.yaml` file:

```yaml
externalDNSName: kubernetes.staging.example.com
createRecordSet: true
hostedZone: staging.example.com
```

If `createRecordSet` is not set to true, the deployer will be responsible for making externalDNSName routable to the controller IP after the cluster is created.

### Multi-AZ Clusters

Kube-aws supports "spreading" a cluster across any number of Availability Zones in a given region.

```yaml
 subnets:
   - availabilityZone: us-west-1a
     instanceCIDR: "10.0.0.0/24"
   - availabilityZone: us-west-1b
     instanceCIDR: "10.0.1.0/24"
```
__A word of caution about EBS and Persistent Volumes__: Any pods deployed to a Multi-AZ cluster must mount EBS volumes via [Persistent Volume Claims](http://kubernetes.io/docs/user-guide/persistent-volumes/#persistentvolumeclaims). Specifying the ID of the EBS volume directly in the pod spec will not work consistently if nodes are spread across multiple zones.

Read more about Kubernetes Multi-AZ cluster support [here](http://kubernetes.io/docs/admin/multiple-zones/).

### Certificates and Keys

`kube-aws render` begins by initializing the TLS infrastructure needed to securely operate Kubernetes. If you have your own key/certificate management system, you can overwrite the generated TLS assets after `kube-aws render`. More information on [Kubernetes certificate generation.][k8s-openssl]

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
  <br/><br/>

* **WorkerCert, WorkerKey**

  The certificate and key granted to the kubelets on worker instances.
  The certificate is shared across all workers, so it must be valid for all worker hostnames.
  This is achievable with the Subject Alternative Name (SAN) `*.*.cluster.internal`, or `*.ec2.internal` if using the us-east-1 AWS region.

* **CACert**

  The certificate authority's TLS certificate is used to sign other certificates in the cluster.

  These assets are stored unencrypted in your `credentials` folder, but are encyrpted using Amazon KMS before being embedded in the CloudFormation template.

  All keys and certs must be PEM-formatted and base64-encoded.

## Render and validate cluster assets

After you have completed your customizations, re-render your assets with the new settings:

```sh
$ kube-aws render
```

The `validate` command check the validity of your changes to the cloud-config userdata files and the CloudFormation stack description.

This is an important step to make sure your stack will launch successfully:

```sh
$ kube-aws validate
```

If your files are valid, you are ready to [launch your cluster][aws-step-3].

[aws-step-1]: kubernetes-on-aws.md
[aws-step-2]: kubernetes-on-aws-render.md
[aws-step-3]: kubernetes-on-aws-launch.md
[k8s-openssl]: openssl.md
[tls-note]: #certificates-and-keys
[route53]: https://aws.amazon.com/route53/
[rdb]: kubelet-wrapper.md#allow-pods-to-use-rbd-volumes
[iscsi]: kubelet-wrapper.md#allow-pods-to-use-iscsi-mounts
[host-dns]: kubelet-wrapper.md#use-the-hosts-dns-configuration
[cluster-logging]: kubelet-wrapper.md#use-the-cluster-logging-add-on
