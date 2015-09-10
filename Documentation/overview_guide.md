# Basic overview

This sets up a basic master with TLS + the ability to add worker nodes. 

## Prepare required keys material and config files

The kubernetes API has various methods for validating clients. All clients, wether it is the kubectl client on your laptop, or a node registering itself with the API share the same authentication mechanism. This guide setups of the API server to use client cert authentication. This means we will show you how to setup a CA and generate the proper credentials 

### establish cluster CA and self-sign a cert

This is the root CA. We use this to create HTTPS keys for the apiserver, as well as for kubectl to be able to connect to the api server

```
openssl genrsa -out ssl/ca-key.pem 2048
openssl req -x509 -new -nodes -key ssl/ca-key.pem -days 10000 -out ssl/ca.pem -subj "/CN=kube-ca"
```

You will want to take care to store the ca-key in a secure location for future use. 

#### create apiserver key and signed cert

This is used for the API servers HTTPS key and cert

```
openssl genrsa -out ssl/apiserver-key.pem 2048
openssl req -new -key ssl/apiserver-key.pem -out ssl/apiserver.csr -subj "/CN=kube-apiserver" -config ssl/apiserver-req.cnf
openssl x509 -req -in ssl/apiserver.csr -CA ssl/ca.pem -CAkey ssl/ca-key.pem -CAcreateserial -out ssl/apiserver.pem -days 365 -extensions v3_req -extfile ssl/apiserver-req.cnf
```
#### create keys for the worker nodes key and signed cert

same as above

#### create admin key for kubectl

This is the key for kubectl account on your local workstation. Same process as above

```
openssl genrsa -out ssl/admin-key.pem 2048
openssl req -new -key ssl/admin-key.pem -out ssl/admin.csr -subj "/CN=kube-admin"
openssl x509 -req -in ssl/admin.csr -CA ssl/ca.pem -CAkey ssl/ca-key.pem -CAcreateserial -out ssl/admin.pem -days 365
```

#### create key for service accounts

This is used by the API Server to create service accounts for pods on deploy. 

```
openssl genrsa -out service-account-private-key.pem 4096
```

## Launch CoreOS instance for master. 

Now you will want to boot a CoreOS instance for use of the master. 

## Configure and install systemd units

    these are the defaults for the vars below. 

```
    if [ -z $ADVERTISE_IP ]; then
        export ADVERTISE_IP=$(awk -F= '/COREOS_PUBLIC_IPV4/ {print $2}' /etc/environment)
    fi
    if [ -z $POD_NETWORK ]; then
        export POD_NETWORK="10.2.0.0/16"
    fi
    if [ -z $ETCD_ENDPOINTS ]; then
        export ETCD_ENDPOINTS="http://127.0.0.1:2379"
    fi
    if [ -z $SERVICE_IP_RANGE ]; then
        export SERVICE_IP_RANGE="10.3.0.0/24"
    fi
    if [ -z $K8S_SERVICE_IP ]; then
        export K8S_SERVICE_IP="10.3.0.1"
    fi
    if [ -z $DNS_SERVICE_IP ]; then
        export DNS_SERVICE_IP="10.3.0.10"
    fi
```

### kubelet.service

This will go in /etcd/systemd/system/kubelet.service

```
[Service]
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
ExecStart=/usr/bin/kubelet \
  --api_servers=http://127.0.0.1:8080 \
  --register-node=${REGISTER_NODE} \
  --allow-privileged=true \
  --config=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster_dns=${DNS_SERVICE_IP} \
  --cluster_domain=cluster.local \
  --cadvisor-port=0
Restart=always
RestartSec=10
```

REGISTER_NODE: "true" if you want this node to be part of the cluster itself. Setting this to true will allow you to have a self contained, single node, cluster, but will have zero availability properties.

ADVERTISE_IP:  the IP that the API server should use to tell clients where to connect. This is useful if you are advertising on a public IP that is not attached to the instance itself. This is common on AWS for example. 

DNS_SERVICE_IP: location of the dns service within the cluster itself. Use 10.3.0.1 unless you have a reason not to. 


## setup etcd

Refer to etcd guide for best practices for etcd. 

reminder: if the master goes down it does not take the cluster down with it. However, completely losing the etcd cluster will lose all the currently deployed kubernetes objects.

### No HA

Intended for trying things out

Just start etcd on node and point everything at this. This can be the same node as the master, but this has obvious HA issues. 

### clustered etcd on the master control plane

Setup a three node master cluster and start etcd directly on all the nodes.

Add the service file stuff here

### Point to an external etcd

This is the ideal option from a production operational perspective, but causes the most resource overhead. It means setup etcd elsewhere, then configure the master to point to it. 

## Setup Flannel

Flannel is needed to setup the proper routes to the worker nodes. Please see flannel docs for configuring flannel for different types of environments. 

Flannel needs an etcd cluster too. by convention it should be the same one as the apiserver, but does not need to be. 

The master nodes need to be able to reach the kubelet meaning they share the same backend network. However, the master does *not* need to be able to get on the service network that is created by kube-proxy. 

## Setup Docker

docker need to be started with the right flags to use the correct network. If you are on AWS you do not need to do anything because the actual routing fabric is managed by VPC. If you are using the built-in overlay, you need to bring up flannel before bringing up docker. This is tricky because flannel is installed with docker, so you need to do early docker stuff. 

## Setup pods for master using kubelet

This is for a single master. Single master is OK because if the master goes away, nothing stops in the cluster-- except new scheduling. However, we have also written a [guide for creating a highly available master].

## Setup pods for worker nodes kubelet


all of these manifests go in /etc/kubernetes/manifests. 

### kube-apiserver.yml

This brings up the actual API server. You need to point it at your ETCD_ENDPOINT (per etcd instructions). SERVICE_IP_RANGE (default above). and ADVERTISE_IP where the apiserver advertises itself, similar issue to kubelet above.

We use the default admission controllers recommended by the kubernetes 1.0 documentation.
NamespaceLifecycle,NamespaceExists,LimitRanger,SecurityContextDeny,ServiceAccount,ResourceQuota

Need to put the keys from above in the above location, per the kube-apiserver.yml file

        - --tls-cert-file=/etc/kubernetes/ssl/apiserver.pem
        - --tls-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem

### kube-scheduler.yml

this connects to the apiserver locally, unauthenticated, no configuration needed. Just use the kube-scheduler as is. 

### kube-controller-manager.yml

Need to put the service-account-private-key.pem from above at /etc/kubernetes/service-account-private-key.pem per the --service-account-private-key-file=/etc/kubernetes/service-account-private-key.pem argument.

## Start kubelet

systemctl start kubelet

You should be able to run "docker ps" and see the containers begin to populate. As soon as docker downloads them the api will come up at localhost:8080.

## Start addons once API is up

The easiest way to do this is with kubectl itself. 

```
kubectl -f base-addons.yml
```

(internal note: we need a base-addons.yml) 

## Configure kubectl locally

You have to create your kubeconfig using the keys created above and the IP of the master. 

## Get started with kubernetes!

Now we need to point to a guide showing how to do your first app on k8s.
