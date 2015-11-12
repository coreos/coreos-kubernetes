## Download the CoreOS image

In this guide, the example virtual machines we are creating are called
`kube-master` and `kube-worker`. They will be backed by a CoreOS image and
stored under `/var/lib/libvirt/images/coreos`. This is not a requirement - feel
free to substitute that path if you use another one.

```
mkdir -p /var/lib/libvirt/images/coreos
cd /var/lib/libvirt/images/coreos
wget http://alpha.release.core-os.net/amd64-usr/current/coreos_production_qemu_image.img.bz2 -O - | bzcat > coreos_production_qemu_image.img
```

## Virtual machine configuration

New create a qcow2 image snapshot for both machines using the command below:
```
cd /var/lib/libvirt/images/coreos
qemu-img create -f qcow2 -b coreos_production_qemu_image.img kube-master.qcow2
qemu-img create -f qcow2 -b coreos_production_qemu_image.img kube-worker.qcow2
```
These images will store the differences between the VM and the base image
separately for each machine.

## Generate TLS keys

See the included script `gentls.sh` for steps. Running the script will generate
a new CA, issue certs for the master, worker, and `kubectl` command line tool.

## Generate ssh keys

It is good practice to use dedicated credentials for accessing guest systems.
Generate a new SSH keypair like this:
```
cd /var/lib/libvirt/images/coreos
ssh-keygen -t rsa -b 2048 -f vm_key
```

## Set up config drive

Now create a config drive file system to configure CoreOS itself. We will use
these to configure the machines and provision TLS artifacts for Kubernetes.

The config drives should contain provisioning scripts and the resources they
need to operate. Assuming you have cloned `coreos-kubernetes` to $CLONEDIR:

For kube-master:
```
cd /var/lib/libvirt/images/coreos
cp -R $CLONEDIR/multi-node/libvirt/kube-master /var/lib/libvirt/images/coreos/
cp $CLONEDIR/multi-node/libvirt/certs/kube-apiserver.tar /var/lib/libvirt/images/coreos/kube-master/openstack/latest/
cp vm_key.pub /var/lib/libvirt/images/coreos/kube-master/openstack/latest/

```

For kube-worker:
```
cd /var/lib/libvirt/images/coreos
cp -R $CLONEDIR/multi-node/libvirt/kube-worker /var/lib/libvirt/images/coreos/
cp $CLONEDIR/multi-node/libvirt/certs/kube-worker.tar /var/lib/libvirt/images/coreos/kube-worker/openstack/latest/
cp vm_key.pub /var/lib/libvirt/images/coreos/kube-worker/openstack/latest/
```

The `user_data` scripts will install the necessary Kubernetes manifests and
assume a certain preconfigured network structure.

## SELinux configuration

If you are using SELinux in enforcing mode, you will need to properly label
these config drive directories.

```
cd /var/lib/libvirt/images/coreos
chcon -R -t svirt_image_t kube-master
chcon -R -t svirt_image_t kube-worker
```

## Network configuration

```
# virsh net-define $CLONEDIR/multi-node/libvirt/network-kubernetes.xml
# virsh net-autostart kubernetes
# virsh net-start kubernetes
```

## Virtual machine startup

kube-master
```
virt-install --connect qemu:///system --import --name kube-master --ram 1024 --vcpus 1 --os-type=linux --os-variant=virtio26 --disk path=/var/lib/libvirt/images/coreos/kube-master.qcow2,format=qcow2,bus=virtio --filesystem /var/lib/libvirt/images/coreos/kube-master/,config-2,type=mount,mode=squash --network bridge=virbrk8s,mac=54:52:00:fe:b3:b0 --vnc --noautoconsole
```

kube-worker
```
virt-install --connect qemu:///system --import --name kube-worker --ram 1024 --vcpus 1 --os-type=linux --os-variant=virtio26 --disk path=/var/lib/libvirt/images/coreos/kube-worker.qcow2,format=qcow2,bus=virtio --filesystem /var/lib/libvirt/images/coreos/kube-worker/,config-2,type=mount,mode=squash --network bridge=virbrk8s,mac=54:52:00:fe:b3:b1 --vnc --noautoconsole
```

## Configure kubectl
Assuming you have cloned `coreos-kubernetes` to $CLONEDIR and run `gentls.sh`:
```
MASTER_HOST=192.168.120.10 CA_CERT=$CLONEDIR/multi-node/libvirt/certs/ca.pem ADMIN_KEY=$CLONEDIR/multi-node/libvirt/certs/admin-key.pem ADMIN_CERT=$CLONEDIR/multi-node/libvirt/certs/admin.pem
kubectl config set-cluster default-cluster --server=https://${MASTER_HOST} --certificate-authority=${CA_CERT}
kubectl config set-credentials default-admin --certificate-authority=${CA_CERT} --client-key=${ADMIN_KEY} --client-certificate=${ADMIN_CERT}
kubectl config set-context default-system --cluster=default-cluster --user=default-admin
kubectl config use-context default-system
```
