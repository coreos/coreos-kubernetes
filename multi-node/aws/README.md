# Kubernetes on AWS

This is the source of the `kube-aws` tool and the installation artifacts used by the official Kubernetes on AWS documentation.
View the full instructions at https://coreos.com/kubernetes/docs/latest/kubernetes-on-aws.html.

## Development

### Build

Run the `./build` script to compile `kube-aws` locally.
This depends on having golang available on your workstation.
The compiled binary will be available at `./bin/kube-aws`.

## Render a new cluster asset directory

Create a yaml file describing your cluster configuration based on [this example](cluster.yaml.example)

Render that configuration into an asset directory.

Feel free to place either `--config` or `--asset-dir` outside the project repository.

Be aware that `--asset-dir` (the asset directory) will contain unencrypted TLS assets after `render` completes. Having these in hand will grant access to your cluster.

```sh
$ ./bin/kube-aws render --config=./my-cluster.yaml --asset-dir=./assets/my-cluster
```

Your config yaml has been rendered to `./assets/my-cluster/cluster.yaml`. At this point, the asset directory contains everything that is needed to create a cluster.

## Create a cluster from asset directory

```sh
$ ./bin/kube-aws up --asset-dir=./assets/my-cluster
```
## How it works

Check out [this diagram](./kube-aws.png)

### Useful Resources

The following links can be useful for development:

- [AWS CloudFormation resource types](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html)

## Contributing

Submit a PR to this repository, following the [contributors guide](../../CONTRIBUTING.md).
The documentation is published from [this source](../../Documentation/kubernetes-on-aws.md).


