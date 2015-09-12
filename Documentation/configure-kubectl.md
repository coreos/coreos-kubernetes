## Configure kubectl

The primary CLI tool used to interact with the Kubernetes API is called `kubectl`.

The following steps should be done from your local workstation to configure `kubectl` to work with your new cluster.

First, download the binary using a command-line tool such as `wget` or `curl`:

```sh
# Replace ${ARCH} with "linux" or "darwin" based on your workstation operating system
$ wget https://storage.googleapis.com/kubernetes-release/release/v1.0.3/bin/${ARCH}/amd64/kubectl
```

After downloading the binary, ensure it is executable and move it into your PATH:

```sh
$ chmod +x kubectl
$ mv kubectl /usr/local/bin/kubectl
```

Configure your local Kubernetes client using the following commands:

* Replace `${MASTER_IP}`
* Replace `${CA_CERT}` with the path to the `ca.pem` created in previous steps
* Replace `${ADMIN_KEY}` with the path to the `admin-key.pem` created in previous steps
* Replace `${ADMIN_CERT}` with the path to the `admin.pem` created in previous steps

```sh
$ kubectl config set-cluster vagrant --server=${MASTER_IP} --certificate-authority=${CA_CERT}
$ kubectl config set-credentials vagrant-admin --certificate-authority=${CA_CERT} --client-key=${ADMIN_KEY} --client-certificate=${ADMIN_CERT}
$ kubectl config set-context vagrant --cluster=vagrant --user=vagrant-admin
$ kubectl config use-context vagrant
```

Check that your client is configured properly by using `kubectl` to inspect your cluster:

```sh
$ kubectl get nodes
NAME          LABELS                               STATUS
X.X.X.X       kubernetes.io/hostname=X.X.X.X       Ready
```

<div class="co-m-docs-next-step">
  <p><strong>Is kubectl working from your local machine?</strong> We're going to install an add-on with it next.</p>
  <a href="deploy-workers.md" class="btn btn-primary btn-icon-right">Yes, ready to deploy add-ons</a>
</div>