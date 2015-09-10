# CoreOS + Kubernetes

This guide will walk you through a manual deployment of a single-master/multi-worker Kubernetes cluster on CoreOS.

## Deployment Options

The following variables will be used throughout this guide. Most of the provided defaults can safely be used, however some values such as `ETCD_ENDPOINTS` and `MASTER_IP` will need to be provided by the deployer.

```
# The IP address of the master node. Worker nodes must be able to reach the master via this IP on port 443. Additionally, external clients (such as an administrator using `kubectl`) will also need access.
MASTER_IP=

# List of etcd servers (http://ip:port), comma separated
ETCD_ENDPOINTS=

# The CIDR network to use for pod IPs.
# Each pod launched in the cluster will be assigned an IP out of this range.
# This network must be routable between all nodes in the cluster. In a default installation, the flannel overlay network will provide routing to this network.
POD_NETWORK=10.2.0.0/16

# The CIDR network to use for service cluster IPs.
# Each service will be assigned a cluster IP out of this range.
# This must not overlap with any IP ranges assigned to the POD_NETWORK, or other existing network infrastructure.
# Routing to these IPs is handled by a kube-proxy service local to each node, and are not required to be routable between nodes.
SERVICE_IP_RANGE=10.3.0.0/24

# The IP address of the Kubernetes API Service
# If the SERVICE_IP_RANGE is changed above, this must be set to the first IP in that range.
K8S_SERVICE_IP=10.3.0.1

# The IP address of the cluster DNS service.
# This IP must be in the range of the SERVICE_IP_RANGE and cannot be the first IP in the range.
# This same IP must be configured on all worker nodes to enable DNS service discovery.
DNS_SERVICE_IP=10.3.0.10
```

## Deploy etcd Cluster

### Single-Node (development)

You can simply start etcd on a CoreOS node:

```
systemctl start etcd2
```

In the rest of this guide use this nodes IP and the url `http://$IP:2379` as the `ETCD_ENDPOINTS`

### Multi-Node (Production)

It is highly recommended that etcd is run as a dedicated cluster separately from Kubernetes components.

Use the [official etcd clustering guide](https://coreos.com/etcd/docs/latest/clustering.html) to decide how best to deploy etcd into your environment.

## Generate Kubernetes TLS Assets

The kubernetes API has various methods for validating clients, and this guide configures the API server to use client cert authentication.

This means it is necessary to have a Certificate Authority and generate the proper credentials. This can be done by generating the necessary assets from existing PKI infrastructure, or linked below are manual OpenSSL instructions to get started.

[OpenSSL Manual Generation](openssl.md)

In the following steps, it is assumed that you will have generated the following TLS assets:

```
# Root CA public key
ca.pem

# API Server public & private keys
apiserver.pem
apiserver-key.pem

# Worker node public & private keys
worker.pem
worker-key.pem

# Cluster Admin public & private keys
admin.pem
admin-key.pem
```

## Deploy Master Node

Boot a single CoreOS node which will be used as the Kubernetes master.

See the [CoreOS Documentation](https://coreos.com/os/docs/latest/) for guides on launching nodes on supported platforms.

### Configure Service Components

#### TLS Assets

Place the keys generated previously in the following locations:

* File: `/etc/kubernetes/ssl/ca.pem`
* File: `/etc/kubernetes/ssl/apiserver.pem`
* File: `/etc/kubernetes/ssl/apiserver-key.pem`

#### Flannel Options

* Create File: `/run/flannel/options.env`
* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

File Contents:

```
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```

#### Docker Config

Require that flanneld is running prior to Docker start.

* Create File: `/etc/systemd/system/docker.service.d/40-flannel.conf`

File Contents:

```
[Unit]
Requires=flanneld.service
After=flanneld.service
```

#### kubelet

* Create File: `/etc/systemd/system/kubelet.service`
* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.
* Replace `${DNS_SERVICE_IP}`

File Contents:

```
[Service]
ExecStart=/usr/bin/kubelet \
  --api_servers=http://127.0.0.1:8080 \
  --register-node=false \
  --allow-privileged=true \
  --config=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster_dns=${DNS_SERVICE_IP} \
  --cluster_domain=cluster.local \
  --cadvisor-port=0
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
```

#### kube-apiserver

* Create File: `/etc/kubernetes/manifests/kube-apiserver.yaml`
* Replace `${ETCD_ENDPOINTS}`
* Replace `${SERVICE_IP_RANGE}`
* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.

File Contents:

```
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-apiserver
    image: gcr.io/google_containers/hyperkube:v1.0.3
    command:
    - /hyperkube
    - apiserver
    - --bind-address=0.0.0.0
    - --etcd_servers=${ETCD_ENDPOINTS}
    - --allow-privileged=true
    - --service-cluster-ip-range=${SERVICE_IP_RANGE}
    - --secure_port=443
    - --advertise-address=${ADVERTISE_IP}
    - --admission-control=NamespaceLifecycle,NamespaceExists,LimitRanger,SecurityContextDeny,ServiceAccount,ResourceQuota
    - --tls-cert-file=/etc/kubernetes/ssl/apiserver.pem
    - --tls-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
    - --client-ca-file=/etc/kubernetes/ssl/ca.pem
    - --service-account-key-file=/etc/kubernetes/ssl/apiserver-key.pem
    ports:
    - containerPort: 443
      hostPort: 443
      name: https
    - containerPort: 7080
      hostPort: 7080
      name: http
    - containerPort: 8080
      hostPort: 8080
      name: local
    volumeMounts:
    - mountPath: /etc/kubernetes/ssl
      name: etckubessl
      readOnly: true
    - mountPath: /etc/ssl
      name: etcssl
      readOnly: true
    - mountPath: /var/ssl
      name: varssl
      readOnly: true
    - mountPath: /etc/openssl
      name: etcopenssl
      readOnly: true
    - mountPath: /etc/pki/tls
      name: etcpkitls
      readOnly: true
  volumes:
  - name: etckubessl
    hostPath:
      path: /etc/kubernetes/ssl
  - hostPath:
      path: /etc/ssl
    name: etcssl
  - hostPath:
      path: /var/ssl
    name: varssl
  - hostPath:
      path: /etc/openssl
    name: etcopenssl
  - hostPath:
      path: /etc/pki/tls
    name: etcpkitls
```

#### kube-proxy

* Create File: `/etc/kubernetes/manifests/kube-proxy.yaml`

File Contents:

```
apiVersion: v1
kind: Pod
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-proxy
    image: gcr.io/google_containers/hyperkube:v1.0.3
    command:
    - /hyperkube
    - proxy
    - --master=http://127.0.0.1:8080
    securityContext:
      privileged: true
    volumeMounts:
      - mountPath: /etc/ssl/certs
        name: "ssl-certs"
  volumes:
    - name: "ssl-certs"
      hostPath:
        path: "/usr/share/ca-certificates"
```

#### kube-controller-manager

* Create File: `/etc/kubernetes/manifests/kube-controller-manager.yaml`

File Contents:

```
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    image: gcr.io/google_containers/hyperkube:v1.0.3
    command:
    - /hyperkube
    - controller-manager
    - --master=http://127.0.0.1:8080
    - --service-account-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
    - --root-ca-file=/etc/kubernetes/ssl/ca.pem
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10252
      initialDelaySeconds: 15
      timeoutSeconds: 1
    volumeMounts:
    - mountPath: /etc/kubernetes/ssl
      name: etckubessl
      readOnly: true
    - mountPath: /etc/ssl
      name: etcssl
      readOnly: true
    - mountPath: /var/ssl
      name: varssl
      readOnly: true
    - mountPath: /etc/openssl
      name: etcopenssl
      readOnly: true
    - mountPath: /etc/pki/tls
      name: etcpkitls
      readOnly: true
  hostNetwork: true
  volumes:
  - name: etckubessl
    hostPath:
      path: /etc/kubernetes/ssl
  - hostPath:
      path: /etc/ssl
    name: etcssl
  - hostPath:
      path: /var/ssl
    name: varssl
  - hostPath:
      path: /etc/openssl
    name: etcopenssl
  - hostPath:
      path: /etc/pki/tls
    name: etcpkitls
```

#### kube-scheduler

* Create File: `/etc/kubernetes/manifests/kube-scheduler.yaml`

File Contents:

```
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-scheduler
    image: gcr.io/google_containers/hyperkube:v1.0.3
    command:
    - /hyperkube
    - scheduler
    - --master=http://127.0.0.1:8080
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10251
      initialDelaySeconds: 15
      timeoutSeconds: 1
```

### Start Services

#### Load Changed Units

```
systemctl reload-daemon
```

#### Configure Flannel Network

* Replace `$POD_NETWORK`
* Replace `$ETCD_SERVER` with one host from `$ETCD_ENDPOINTS`

```
curl -X PUT -d "value={\"Network\":\"$POD_NETWORK\"}" "$ETCD_SERVER/v2/keys/coreos.com/network/config"
```

#### Start Kubelet

```
systemctl start kubelet
```

```
# Automatically start Kubelet at boot
systemctl enable kubelet
```

## Deploy Worker Node(s)

Boot one or more CoreOS nodes which will be used as Kubernetes Workers.

See the [CoreOS Documentation](https://coreos.com/os/docs/latest/) for guides on launching nodes on supported platforms.

### Configure Service Components

#### TLS Assets

Place the TLS keypairs generated previously in the following locations:

* File: `/etc/kubernetes/ssl/ca.pem`
* File: `/etc/kubernetes/ssl/worker.pem`
* File: `/etc/kubernetes/ssl/worker-key.pem`

#### Flannel Options

* Create File `/run/flannel/options.env`
* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

File Contents:

```
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```

#### Docker Config

Require that flanneld is running prior to Docker start.

* Create File: `/etc/systemd/system/docker.service.d/40-flannel.conf`

File Contents:

```
[Unit]
Requires=flanneld.service
After=flanneld.service
```

#### kubelet

* Create File: `/etc/systemd/system/kubelet.service`
* Replace `${MASTER_IP}`
* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.
* Replace `${DNS_SERVICE_IP}`

File Contents:

```
[Service]
ExecStart=/usr/bin/kubelet \
  --api_servers=https://${MASTER_IP} \
  --register-node=true \
  --allow-privileged=true \
  --config=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster_dns=${DNS_SERVICE_IP} \
  --cluster_domain=cluster.local \
  --kubeconfig=/etc/kubernetes/worker-kubeconfig.yaml \
  --tls-cert-file=/etc/kubernetes/ssl/worker.pem \
  --tls-private-key-file=/etc/kubernetes/ssl/worker-key.pem \
  --cadvisor-port=0
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
```

#### kube-proxy

* Create File: `/etc/kubernetes/manifests/kube-proxy.yaml`
* Replace `${MASTER_IP}

File Contents:

```
apiVersion: v1
kind: Pod
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-proxy
    image: gcr.io/google_containers/hyperkube:v1.0.3
    command:
    - /hyperkube
    - proxy
    - --master=https://${MASTER_IP}
    - --kubeconfig=/etc/kubernetes/worker-kubeconfig.yaml
    securityContext:
      privileged: true
    volumeMounts:
      - mountPath: /etc/ssl/certs
        name: "ssl-certs"
      - mountPath: /etc/kubernetes/worker-kubeconfig.yaml
        name: "kubeconfig"
        readOnly: true
      - mountPath: /etc/kubernetes/ssl
        name: "etc-kube-ssl"
        readOnly: true
  volumes:
    - name: "ssl-certs"
      hostPath:
        path: "/usr/share/ca-certificates"
    - name: "kubeconfig"
      hostPath:
        path: "/etc/kubernetes/worker-kubeconfig.yaml"
    - name: "etc-kube-ssl"
      hostPath:
        path: "/etc/kubernetes/ssl"
```

#### kubeconfig

* CreateFile: `/etc/kubernetes/worker-kubeconfig.yaml`

File Contents:

```
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    certificate-authority: /etc/kubernetes/ssl/ca.pem
users:
- name: kubelet
  user:
    client-certificate: /etc/kubernetes/ssl/worker.pem
    client-key: /etc/kubernetes/ssl/worker-key.pem
contexts:
- context:
    cluster: local
    user: kubelet
  name: kubelet-context
current-context: kubelet-context
```

### Start Services

#### Load Changed Units

```
systemctl reload-daemon
```

#### Start Kubelet

```
systemctl start kubelet
```

```
# Automatically start Kubelet at boot
systemctl enable kubelet
```

## Configure kubectl

The primary CLI tool used to interact with the Kubernetes API is called `kubectl`.

The following steps should be done from your local workstation.

First, download the binary using a command-line tool such as `wget` or `curl`:

```
# Replace ${ARCH} with "linux" or "darwin" based on your workstation operating system
wget https://storage.googleapis.com/kubernetes-release/release/v1.0.3/bin/${ARCH}/amd64/kubectl
```

After downloading the binary, ensure it is executable and move it into your PATH:

```
chmod +x kubectl
mv kubectl /usr/local/bin/kubectl
```

Configure your local Kubernetes client using the following commands:

* Replace `${MASTER_IP}`
* Replace `${CA_CERT}` with the path to the `ca.pem` created in previous steps
* Replace `${ADMIN_KEY}` with the path to the `admin-key.pem` created in previous steps
* Replace `${ADMIN_CERT}` with the path to the `admin.pem` created in previous steps

```
kubectl config set-cluster vagrant --server=${MASTER_IP} --certificate-authority=${CA_CERT}
kubectl config set-credentials vagrant-admin --certificate-authority=${CA_CERT} --client-key=${ADMIN_KEY} --client-certificate=${ADMIN_CERT}
kubectl config set-context vagrant --cluster=vagrant --user=vagrant-admin
kubectl config use-context vagrant
```

Check that your client is configured properly by using `kubectl` to inspect your cluster:

```
% kubectl get nodes
NAME          LABELS                               STATUS
X.X.X.X       kubernetes.io/hostname=X.X.X.X       Ready
```

## Deploy DNS Addon

Save the dns-addon file contents below, then create using kubectl:

```
kubectl create -f dns-addon.yaml
```

* Create File: dns-addon.yaml
* Replace `${DNS_SERVICE_IP}`

File Contents:

```
apiVersion: v1
kind: Namespace
metadata:
  name: kube-system

---

apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "KubeDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: ${DNS_SERVICE_IP}
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP

---

apiVersion: v1
kind: ReplicationController
metadata:
  name: kube-dns-v9
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    version: v9
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 1
  selector:
    k8s-app: kube-dns
    version: v9
  template:
    metadata:
      labels:
        k8s-app: kube-dns
        version: v9
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
      - name: etcd
        image: gcr.io/google_containers/etcd:2.0.9
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        command:
        - /usr/local/bin/etcd
        - -data-dir
        - /var/etcd/data
        - -listen-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -advertise-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -initial-cluster-token
        - skydns-etcd
        volumeMounts:
        - name: etcd-storage
          mountPath: /var/etcd/data
      - name: kube2sky
        image: gcr.io/google_containers/kube2sky:1.11
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        args:
        # command = "/kube2sky"
        - -domain=cluster.local
      - name: skydns
        image: gcr.io/google_containers/skydns:2015-03-11-001
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        args:
        # command = "/skydns"
        - -machines=http://localhost:4001
        - -addr=0.0.0.0:53
        - -domain=cluster.local.
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 1
          timeoutSeconds: 5
      - name: healthz
        image: gcr.io/google_containers/exechealthz:1.0
        resources:
          limits:
            cpu: 10m
            memory: 20Mi
        args:
        - -cmd=nslookup kubernetes.default.svc.cluster.local localhost >/dev/null
        - -port=8080
        ports:
        - containerPort: 8080
          protocol: TCP
      volumes:
      - name: etcd-storage
        emptyDir: {}
      dnsPolicy: Default
```

## Next Steps: Deploy an Application

Now that you have a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.

Start with a [multi-tier web application](http://kubernetes.io/v1.0/examples/guestbook-go/README.html) from the official Kubernetes documentation to visualize how the various Kubernetes components fit together.

