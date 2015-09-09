# Basic overview

This sets up a basic master, with up to X worker nodes.

## Prepare required keys material and config files

### establish cluster CA and self-sign a cert

This is the root CA. We use this to create HTTPS keys for the apiserver, as well as for kubectl to be able to connect to the api server

```
openssl genrsa -out ssl/ca-key.pem 2048
openssl req -x509 -new -nodes -key ssl/ca-key.pem -days 10000 -out ssl/ca.pem -subj "/CN=kube-ca"
```

#### create apiserver key and signed cert

This is used for the API servers HTTPS key and cert

```
openssl genrsa -out ssl/apiserver-key.pem 2048
openssl req -new -key ssl/apiserver-key.pem -out ssl/apiserver.csr -subj "/CN=kube-apiserver" -config ssl/apiserver-req.cnf
openssl x509 -req -in ssl/apiserver.csr -CA ssl/ca.pem -CAkey ssl/ca-key.pem -CAcreateserial -out ssl/apiserver.pem -days 365 -extensions v3_req -extfile ssl/apiserver-req.cnf
```

#### create admin key for kubectl

This is the key for kubectl account on your local workstation.

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

REGISTER_NODE: "true" if you want this node to be part of the cluster itself
ADVERTISE_IP:  the IP that the API server should use to tell clients where to connect. This is useful if you are advertising on a public IP that is not attached to the instance itself. This is common on AWS for example. 
DNS_SERVICE_IP: location of the dns service within the cluster itself. Use suggested default above. 


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

## setup etcd

## Setup Flannel

## Setup Docker

## Setup pods for master using kubelet

This is for a single master. Single master is OK because if the master goes away, nothing stops in the cluster-- except new scheduling. However, we have also written a [guide for creating a highly available master].

## Setup pods for worker nodes kubelet

## Start kubelet

## Start addons once API is up

## Configure kubectl locally
