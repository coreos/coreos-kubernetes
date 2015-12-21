# Kubernetes on AWS

This is the source of the `kube-aws` tool and the installation artifacts used by the official Kubernetes on AWS documentation.
View the full instructions at [Kubernetes on AWS](https://coreos.com/kubernetes/docs/latest/kubernetes-on-aws.html.)

## Development

### Build locally (requires Go installation)

Run the `./build` script to compile `kube-aws` locally.
This depends on having golang available on your workstation.
The compiled binary will be available at `./bin/kube-aws`.

### Build a Docker image :whale:

- Configure AWS credentials using the credentials.example and rename to just **credentials** - for help, see the [AWS CLI Guide](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html#cli-config-file)
```bash
[default]
aws_access_key_id=###
aws_secret_access_key=###
region=###
```
- Configure a cluster.yaml file using the cluster.yaml.example - for help, see the [Kube AWS Cluster Config](https://coreos.com/kubernetes/docs/latest/kubernetes-on-aws.html#kube-aws-cluster-config)
- Build the image using docker
```bash
$ docker build -t kube-aws-docker .
```
- This will result in a runnable image with a size of just 23MB.
```
kube-aws-docker     latest              57d71b91722f        About a minute ago   23.15 MB
```
- Use the image to run kube-aws commands
```bash
$ docker run --rm kube-aws-docker kube-aws help
```

# NOTE

**Please be careful not to push your AWS credentials to Github! :scream:**

### Custom Kubernetes Manifests

You may deploy a cluster using a custom CloudFormation template, Kubernetes manifests and install scripts using the `artifactURL` option in your cluster config.

For example, you might upload a modified set of manifests to a custom S3 bucket (making the files publicly-readable) using the following commands:

```
$ kube-aws render --output=artifacts/template.json
$ aws s3 cp --recursive --acl=public-read artifacts/ s3://<bucket>/
```

Then, simply create a cluster using `artifactURL: https://<bucket>.s3.amazonaws.com`.

### Useful Resources

The following links can be useful for development:

- [AWS CloudFormation resource types](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html)

## Contributing

Submit a PR to this repository, following the [contributors guide](../../CONTRIBUTING.md).
The documentation is published from [this source](../../Documentation/kubernetes-on-aws.md).

