# Kubernetes on CoreOS Generic Install Scripts

These scripts setup Kubernetes on CoreOS in a similar way to other tools in the repo. The notable difference is that these scripts are intended to be platform agnostic and thus don't automatically setup the TLS assets on each host beforehand. Instead, the contract with these scripts is that first you setup TLS assets on each host manually. Then you copy the `controller-install.sh` to the master node and `worker-install.sh` to any worker nodes. Then you run each script, wait for completetion and you should have a running cluster!

While we provide these scripts and test them through the multi-node Vagrant setup, recommend using a platform specific install method if available. If you are installing to bare-metal, you might find our [baremetal repo](https://github.com/coreos/coreos-baremetal) more appropriate.

### Setting up TLS assets

Use the scripts at https://github.com/coreos/coreos-kubernetes/tree/master/lib to generate the assets for each node. Place them under `/etc/kubernetes/ssl`.


