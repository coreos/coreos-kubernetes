# Setting up kubectl

Use kubectl to interact with the Kubernetes API and with the cluster’s shared state. Download kubectl from the [Tectonic Console][tectonic-kubectl], or from the Kubernetes release artifact site with the cURL tool.

Use `curl` to fetch the Linux kubectl binary:

```
$ curl -O https://storage.googleapis.com/kubernetes-release/release/v1.7.5/bin/linux/amd64/kubectl
```

Or, to fetch the macOS binary:

```
$ curl -O https://storage.googleapis.com/kubernetes-release/release/v1.7.5/bin/darwin/amd64/kubectl
```

After downloading the binary, ensure it is executable and move it into your PATH:

```
$ chmod +x kubectl
$ sudo mv kubectl /usr/local/bin/kubectl
```

Terraform generated a set of files, including a `kubeconfig`, which specifies the credentials for your cluster. Once logged in to the Tectonic Console, other users can download their own pre-generated kubeconfigs.

As the first admin, you have a special “root” kubeconfig. Configure `kubectl` to use this file:

```
$ export KUBECONFIG=/path/to/installer/generated/auth/kubeconfig
```

Test that it works by getting cluster info:

```
$ kubectl cluster-info
```

You should see output about the addresses of Kubernetes master, Heapster, and KubeDNS. This proves that the API is running and healthy.

Any connection errors will indicate that your cluster is not yet done bootstrapping.

For more information on troubleshooting cluster errors, see [triaging a cluster][triaging-cluster].


[triaging-cluster]: https://coreos.com/tectonic/docs/latest/troubleshooting/troubleshooting.html
[tectonic-kubectl]: https://coreos.com/tectonic/docs/latest/tutorials/aws/first-app.html#configuring-credentials
