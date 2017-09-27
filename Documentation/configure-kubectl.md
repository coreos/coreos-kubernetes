# Setting up kubectl

<div class="k8s-on-tectonic">
<p class="k8s-on-tectonic-description">This repo is not in alignment with current versions of Kubernetes, and will not be active in the future. The CoreOS Kubernetes documentation has been moved to the <a href="https://github.com/coreos/tectonic-docs/tree/master/Documentation">tectonic-docs repo</a>, where it will be published and updated.</p>

<p class="k8s-on-tectonic-description">For tested, maintained, and production-ready Kubernetes instructions, see our <a href="https://coreos.com/tectonic/docs/latest/install/aws/index.html">Tectonic Installer documentation</a>. The Tectonic Installer provides a Terraform-based Kubernetes installation. It is open source, uses upstream Kubernetes and can be easily customized.</p>
</div>

`kubectl` is a command-line program for interacting with the Kubernetes API. The following steps should be done from a local workstation to configure `kubectl` to work with a new cluster.

To quickly launch a cluster, follow these guides for [AWS][kube-aws], [Vagrant][vagrant-multi] or [full step-by-step][manual] instructions.

[kube-aws]: https://github.com/coreos/kube-aws/blob/master/README.md
[vagrant-multi]: kubernetes-on-vagrant-single.md
[manual]: getting-started.md

## Download the kubectl Executable

Download `kubectl` from the Kubernetes release artifact site with the `curl` tool.

The linux `kubectl` binary can be fetched with a command like:

```sh
$ curl -O https://storage.googleapis.com/kubernetes-release/release/v1.5.4/bin/linux/amd64/kubectl
```

On an OS X workstation, replace `linux` in the URL above with `darwin`:

```sh
$ curl -O https://storage.googleapis.com/kubernetes-release/release/v1.5.4/bin/darwin/amd64/kubectl
```

After downloading the binary, ensure it is executable and move it into your `PATH`:

```sh
$ chmod +x kubectl
$ mv kubectl /usr/local/bin/kubectl
```

## Configure kubectl

Configure `kubectl` to connect to the target cluster using the following commands, replacing several values as indicated:

* Replace `${MASTER_HOST}` with the master node address or name used in previous steps
* Replace `${CA_CERT}` with the absolute path to the `ca.pem` created in previous steps
* Replace `${ADMIN_KEY}` with the absolute path to the `admin-key.pem` created in previous steps
* Replace `${ADMIN_CERT}` with the absolute path to the `admin.pem` created in previous steps

```sh
$ kubectl config set-cluster default-cluster --server=https://${MASTER_HOST} --certificate-authority=${CA_CERT}
$ kubectl config set-credentials default-admin --certificate-authority=${CA_CERT} --client-key=${ADMIN_KEY} --client-certificate=${ADMIN_CERT}
$ kubectl config set-context default-system --cluster=default-cluster --user=default-admin
$ kubectl config use-context default-system
```

## Verify kubectl Configuration and Connection

Check that the client is configured properly by using `kubectl` to inspect the cluster:

```sh
$ kubectl get nodes
NAME          LABELS                               STATUS
X.X.X.X       kubernetes.io/hostname=X.X.X.X       Ready
```

<div class="co-m-docs-next-step">
  <p><strong>Is kubectl working from your local machine?</strong> We're going to install an add-on with it next.</p>
  <a href="deploy-addons.md" class="btn btn-primary btn-icon-right" data-category="Docs Next" data-event="Kubernetes: Addons">Yes, ready to deploy add-ons</a>
</div>
