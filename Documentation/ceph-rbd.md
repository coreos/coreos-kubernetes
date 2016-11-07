# Ceph rbd storage

The single and multi-node examples both allow you to mount a volume backed by ceph rbd. 

Rook provides the deployment of ceph that makes it very simple to setup the rbd volume.

# Start the Kubernetes cluster with Rook

- `vagrant up` in either the single or multi-node folders to start kubernetes and rook.
- Configure your kubectl client to connect to the kubernetes cluster. This configuration is only required once even if you vagrant destroy and bring up a new instance.

```
kubectl config set-cluster vagrant-single-cluster --server=https://172.17.4.99:443 --certificate-authority=${PWD}/ssl/ca.pem
kubectl config set-credentials vagrant-single-admin --certificate-authority=${PWD}/ssl/ca.pem --client-key=${PWD}/ssl/admin-key.pem --client-certificate=${PWD}/ssl/admin.pem
kubectl config set-context vagrant-single --cluster=vagrant-single-cluster --user=vagrant-single-admin
kubectl config use-context vagrant-single
```

Verify that the cluster is up. It will likely take several minutes. For example, kubectl commands should return successfully.

```
kubectl get nodes
```

Download the [rook client](https://github.com/rook/rook/releases/) from the latest release. Extract `rook` for your platform.

Create the rbd image:

```
rook --api-server-endpoint 172.17.4.99:8124 block create --name demoblock --size 1234567890
```

# Mounting the pod with an rbd volume

Start the example pod in either the single or multi-node folder. The sample is modified from the [kubernetes rbd](https://github.com/kubernetes/kubernetes/tree/master/examples/volumes/rbd) example.

```
kubectl create -f rbd.json
kubectl get pods
```

Inspect the system to see the ceph rbd volume mounted

```
vagrant ssh
mount | grep rbd
docker inspect <container-id>
```
