# Kubernetes Conformance Tests

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
