# Cluster TLS using OpenSSL

This guide will walk you through generating Kubernetes TLS assets using OpenSSL.

This is provided as a proof-of-concept guide to get started with Kubernetes client certificate authentication.

## Deployment Options

The following variables will be used throughout this guide. The default for `K8S_SERVICE_IP` can safely be used, however `MASTER_HOST` will need to be customized to your infrastructure.

**MASTER_HOST**=_no default_

The address of the master node. In most cases this will be the publicly routable IP or hostname of the master cluster. Worker nodes must be able to reach the master node(s) via this address on port 443. Additionally, external clients (such as an administrator using `kubectl`) will also need access, since this will run the Kubernetes API endpoint.

If you will be running a highly-available control-plane consisting of multiple master nodes, then `MASTER_HOST` will ideally be a network load balancer that sits in front of the master nodes. Alternatively, a DNS name can be configured which will resolve to the master node IPs. In either case, the certificates which are generated below need to have the correct CommonName and/or SubjectAlternateNames.

<hr/>

**K8S_SERVICE_IP**=10.3.0.1

The IP address of the Kubernetes API Service. The `K8S_SERVICE_IP` will be the first IP in the `SERVICE_IP_RANGE` discussed in the [deployment guide][deployment-guide]. The first IP in the default range of 10.3.0.0/24 will be 10.3.0.1. If the SERVICE_IP_RANGE was changed from the default, this value must be updated as well.

## Create a Cluster Root CA

First, we need to create a new certificate authority which will be used to sign the rest of our certificates.

```sh
$ openssl genrsa -out ca-key.pem 2048
$ openssl req -x509 -new -nodes -key ca-key.pem -days 10000 -out ca.pem -subj "/CN=kube-ca"
```

**You need to store the CA keypair in a secure location for future use.**

## Kubernetes API Server Keypair

### OpenSSL Config

This is a minimal openssl config which will be used when creating the api-server certificate. We need to create a configuration file since some of the options we need to use can't be specified as flags. Create `openssl.cnf` on your local machine and replace the following values:

* Replace `${K8S_SERVICE_IP}`
* Replace `${MASTER_HOST}`

**openssl.cnf**

```
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = kubernetes
DNS.2 = kubernetes.default
IP.1 = ${K8S_SERVICE_IP}
IP.2 = ${MASTER_HOST}
```

If you are deploying multiple master nodes in an HA configuration, you may need to add additional IP or DNS SubjectAltNames. What is configured depends on how worker nodes and `kubectl` users will be contact the master nodes (directly, via loadbalancer, via DNS name).

Example:

```
DNS.3 = ${MASTER_DNS_NAME}
IP.3 = ${MASTER_IP}
IP.4 = ${MASTER_LOADBALANCER_IP}
```

## Generate the API Server Keypair

Using the above `openssl.cnf`, create the api-server keypair:

```sh
$ openssl genrsa -out apiserver-key.pem 2048
$ openssl req -new -key apiserver-key.pem -out apiserver.csr -subj "/CN=kube-apiserver" -config openssl.cnf
$ openssl x509 -req -in apiserver.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out apiserver.pem -days 365 -extensions v3_req -extfile openssl.cnf
```

## Generate the Kubernetes Worker Keypair

```sh
$ openssl genrsa -out worker-key.pem 2048
$ openssl req -new -key worker-key.pem -out worker.csr -subj "/CN=kube-worker"
$ openssl x509 -req -in worker.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out worker.pem -days 365
```

## Generate the Cluster Administrator Keypair

```sh
$ openssl genrsa -out admin-key.pem 2048
$ openssl req -new -key admin-key.pem -out admin.csr -subj "/CN=kube-admin"
$ openssl x509 -req -in admin.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out admin.pem -days 365
```

You are now ready to return to the [deployment guide][deployment-guide] and configure your Master machine, Workers, and `kubectl` on your local machine.

[deployment-guide]: getting-started.md
