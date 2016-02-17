# Kubernetes on AWS

This is the source of the `kube-aws` tool and the installation artifacts used by the official Kubernetes on AWS documentation.
View the full instructions at https://coreos.com/kubernetes/docs/latest/kubernetes-on-aws.html.

## Development

### Download pre-built binary

```sh
wget https://<binary-url>
# check checksum
chmod +x ./kube-aws
sudo mv kube-aws /usr/bin/
```

### Build

Run the `./build` script to compile `kube-aws` locally.

This depends on having:
* golang >= 1.5
* glide package manager

The compiled binary will be available at `./bin/kube-aws`.

## Initialize an asset directory
```sh
$ mkdir my-cluster
$ cd ./my-cluster
$ kube-aws init --cluster-name=my-cluster-name --external-dns-name=my-cluster-endpoint --region=us-west-1 --key-name=key-pair-name
```

There will now be a ./cluster.yaml file in the asset directory.

## Render contents of the asset directory

```sh
$ kube-aws render
```
You now have a default-configured cluster that is ready to launch.

You can now customize your cluster by editing files:
* ./cluster.yaml (common case)
* `cloud-config/` directory (userdata files)
* stack-template.json
* `credentials/` directory

You can also now check the `./my-cluster` asset directory into version control if you desire. The contents of this directory are your reproducible cluster assets. Please take care not to commit the `./my-cluster/credentials` directory, as it contains your TLS secrets. If you're using git, the `credentials` directory will already be ignored for you.

## Validate your cluster assets

The `validate` command check the validity of the cloud-config userdata files and the cloudformation stack description.

```sh
$ kube-aws validate
```

## Create a cluster from asset directory

```sh
$ kube-aws up
```

This command can take a while.

## Access the cluster

```sh
$ kubectl --kubeconfig=./credentials/kubeconfig get nodes
```

It can take some time after `kube-aws up` completes before the cluster is available. Until then, you'll get a `connection refused` error.

## Update the cluster

After modifying your `cluster.yaml` file (or any of the other asset files), you can attempt to update the cloudformation stack.

*Caveats*
* updates that involve the controller will wipe-away etcd state, which in turn will wipe out kubernetes cluster state.
* updates do not currently succeed if you change some of the "physical" networking options. (vpcCidr is an example).
* the update procedure involves replacing ec2 instances without coordinating with the Kubernetes apiserver. This can (and probably will) produce cluster downtime

```sh
$ kube-aws up --update
```

### Updating SSL assets

* Create a temporary directory and run `kube-aws render`.
* Copy the `./credentials` directory to your "real" assets directory (overwriting the original `credentials` directory)
* Run `kube-aws up --update` in the "real" assets directory. This will propagate the newly generated TLS assets to your cluster.

### Useful Resources

The following links can be useful for development:

- [AWS CloudFormation resource types](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html)

## Contributing

Submit a PR to this repository, following the [contributors guide](../../CONTRIBUTING.md).
The documentation is published from [this source](../../Documentation/kubernetes-on-aws.md).


