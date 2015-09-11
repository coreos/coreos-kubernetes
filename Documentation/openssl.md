# Cluster TLS using OpenSSL

This guide will walk you through generating Kubernetes TLS assets using OpenSSL.

This is provided as a proof-of-concept guide to get started with Kubernetes client certificate authentication.

## Create a Cluster Root CA

First, we need to create a new certificate authority which will be used to sign the rest of our certificates.

```
openssl genrsa -out ca-key.pem 2048
openssl req -x509 -new -nodes -key ca-key.pem -days 10000 -out ca.pem -subj "/CN=kube-ca"
```

You need to store the CA keypair in a secure location for future use.

## Kubernetes API Server Keypair

### OpenSSL Config

This is a minimal openssl config which will be used when creating the api-server certificate. We need to create a configuration file since some of the options we need to use can't be specified as flags. Create `openssl.cnf` on your local machine and replace the following values:

* Replace `${K8S_SERVICE_IP}`
* Replace `${MASTER_IP}`

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
IP.2 = ${MASTER_IP}
```

## Generate the API Server Keypair

Using the above `openssl.cnf`, create the api-server keypair:

```sh
$ openssl genrsa -out apiserver-key.pem 2048
$ openssl req -new -key apiserver-key.pem -out apiserver.csr -subj "/CN=kube-apiserver" -config openssl.cnf
$ openssl x509 -req -in apiserver.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out apiserver.pem -days 365 -extensions v3_req -extfile openssl.cnf
```

## Generate the Kubernetes Worker Keypair

```
$ openssl genrsa -out worker-key.pem 2048
$ openssl req -new -key worker-key.pem -out worker.csr -subj "/CN=kube-worker"
$ openssl x509 -req -in worker.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out worker.pem -days 365
```

## Generate the Cluster Administrator Keypair

```
$ openssl genrsa -out admin-key.pem 2048
$ openssl req -new -key admin-key.pem -out admin.csr -subj "/CN=kube-admin"
$ openssl x509 -req -in admin.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out admin.pem -days 365
```

You are now ready to return to the [deployment guide](overview_guide.md) and configure your Master machine, Workers, and `kubectl` on your local machine.