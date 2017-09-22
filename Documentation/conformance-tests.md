# Kubernetes Conformance Tests

<div class="k8s-on-tectonic">
<p class="k8s-on-tectonic-description">This repo is not in alignment with current versions of Kubernetes, and will not be active in the future. The CoreOS Kubernetes documentation has been moved to the <a href="https://github.com/coreos/tectonic-docs/tree/master/Documentation">tectonic-docs repo</a>, where it will be published and updated.</p>

<p class="k8s-on-tectonic-description">For tested, maintained, and production-ready Kubernetes instructions, see our <a href="https://coreos.com/tectonic/docs/latest/install/aws/index.html">Tectonic Installer documentation</a>. The Tectonic Installer provides a Terraform-based Kubernetes installation. It is open source, uses upstream Kubernetes and can be easily customized.</p>
</div>

The conformance test checks that a kubernetes installation supports a minimum required feature set.

## Prerequisites

You will need to have a Kubernetes cluster already running.

Make sure only the essential pods are running, and there are no failed or pending pods. If you are testing a small / development cluster, you may need to increase the node memory or tests could inconsistently fail (e.g. 2048mb for single-node vagrant).

## Running the Tests

If you are running the conformance tests against a vagrant cluster, you can use the `conformance-test.sh` script located either in the `single-node` or `multi-node/vagrant` directories.

To test a running cluster:

First, clone a copy of this repository to a host with ssh access to the Kubernetes cluster.

```sh
$ git clone https://github.com/coreos/coreos-kubernetes
```

Then run the `contrib/conformance-test.sh` helper script, replacing the <ssh-host> <ssh-port> and <path-to-ssh-key>

```sh
$ cd coreos-kubernets/contrib
$ ./conformance-test.sh <ssh-host> <ssh-port> <path-to-ssh-key>
```
