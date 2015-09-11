# CoreOS + Kubernetes

This guide will walk you through a deployment of a single-master/multi-worker Kubernetes cluster on CoreOS. We're going to configure or deploy:

- an etcd cluster for Kubernetes to use
- generate the required certificates for communication between Kubernetes components
- deploy our Master node
- deploy our Worker nodes
- configure `kubectl` to work with our cluster
- deploy the DNS add-on

Working through this guide may take you a few hours, but it will give you good understanding of the moving pieces of your cluster and set you up for success in the long run. Let's get started.

## Deployment Options

The following variables will be used throughout this guide. Most of the provided defaults can safely be used, however some values such as `ETCD_ENDPOINTS` and `MASTER_IP` will need to be customized to your infrastructure.

**MASTER_IP**=_no default_

The IP address of the master node. Worker nodes must be able to reach the master via this IP on port 443. Additionally, external clients (such as an administrator using `kubectl`) will also need access, since this will run the Kubernetes API endpoint.

<hr/>

**ETCD_ENDPOINTS**=_no default_

List of etcd machines (`http://ip:port`), comma separated. If you're running a cluster of 5 machines, list them all here.

<hr/>

**POD_NETWORK**=10.2.0.0/16

The CIDR network to use for pod IPs.
Each pod launched in the cluster will be assigned an IP out of this range.
This network must be routable between all nodes in the cluster. In a default installation, the flannel overlay network will provide routing to this network.

<hr/>

**SERVICE_IP_RANGE**=10.3.0.0/24

The CIDR network to use for service cluster IPs. Each service will be assigned a cluster IP out of this range. This must not overlap with any IP ranges assigned to the POD_NETWORK, or other existing network infrastructure. Routing to these IPs is handled by a kube-proxy service local to each node, and are not required to be routable between nodes.

<hr/>

**K8S_SERVICE_IP**=10.3.0.1

The IP address of the Kubernetes API Service. If the SERVICE_IP_RANGE is changed above, this must be set to the first IP in that range.

<hr/>

**DNS_SERVICE_IP**=10.3.0.10

The IP address of the cluster DNS service. This IP must be in the range of the SERVICE_IP_RANGE and cannot be the first IP in the range. This same IP must be configured on all worker nodes to enable DNS service discovery.

## Deploy etcd Cluster

Kubernetes uses etcd for data storage and for cluster consensus between different software components. Your etcd cluster will be heavily utilized since all objects storing within and every scheduling decision is recorded. It's recommended that you run a multi-machine cluster on dedicated hardware (with fast disks) to gain maximum performance and reliability of this important part of your cluster. For development environments, a single etcd is ok.

### Single-Node (Development)

You can simply start etcd via [cloud-config][cloud-config-etcd] when you create your CoreOS machine or start it manually:

```
$ sudo systemctl start etcd2
```

To ensure etcd starts after a reboot, enable it too:

```sh
$ sudo systemctl enable etcd2
```

Record the IP address of an network interface on this machine that is reachable from your Kubernetes master, which will be configured below. In the rest of this guide, that IP in the form `http://$IP:2379` as the `ETCD_ENDPOINTS`.

[cloud-config-etcd]: https://coreos.com/os/docs/latest/cloud-config.html#etcd2

### Multi-Node (Production)

It is highly recommended that etcd is run as a dedicated cluster separately from Kubernetes components.

Use the [official etcd clustering guide](https://coreos.com/etcd/docs/latest/clustering.html) to decide how best to deploy etcd into your environment.

## Generate Kubernetes TLS Assets

The Kubernetes API has various methods for validating clients &mdash; this guide will configure the API server to use client cert authentication.

This means it is necessary to have a Certificate Authority and generate the proper credentials. This can be done by generating the necessary assets from existing PKI infrastructure, or follow the OpenSSL instructions to create everything needed.

[OpenSSL Manual Generation](openssl.md)

In the following steps, it is assumed that you will have generated the following TLS assets:

**Root CA Public Key**

ca.pem

<hr/>

**API Server Public & Private Keys**

apiserver.pem

apiserver-key.pem

<hr/>

**Worker Node Public & Private Keys**

worker.pem

worker-key.pem

<hr/>

**Cluster Admin Public & Private Keys**

admin.pem

admin-key.pem

## Deploy Kubernetes Master Machine

Boot a single CoreOS machine which will be used as the Kubernetes master.

See the [CoreOS Documentation](https://coreos.com/os/docs/latest/) for guides on launching nodes on supported platforms.

Manual configuration of the required Master services is explained below, but most of the configuration could also be done with cloud-config, aside from placing the TLS assets on disk. These secrets shouldn't be stored in cloud-config for enhanced security.

### Configure Service Components

#### TLS Assets

Place the keys generated previously in the following locations:

* File: `/etc/kubernetes/ssl/ca.pem`
* File: `/etc/kubernetes/ssl/apiserver.pem`
* File: `/etc/kubernetes/ssl/apiserver-key.pem`

#### Flannel Configuration

[Flannel][flannel-docs] provides a key Kubernetes networking capability &mdash; a software-defined overlay network to give each Kubernetes [Service][service-overview] or [Pod][pod-overview] its own IP address.

Flannel stores local configuration in `/run/flannel/options.env` and cluster-level configuration in etcd. Create this file and edit the contents:

* Replace `${ADVERTISE_IP}` with this machine's publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

**/run/flannel/options.env**

```yml
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```

[flannel-docs]: https://coreos.com/flannel/docs/latest/
[pod-overview]: https://coreos.com/kubernetes/docs/latest/pods.html
[service-overview]: https://coreos.com/kubernetes/docs/latest/services.html

#### Docker Configuration

In order for Flannel networking in the cluster, Docker needs to be configured to use it. All we need to do is require that flanneld is running prior to Docker starting.

We're going to do this with a [systemd drop-in][dropins], which is a method for appending or overriding parameters of a systemd unit. In this case we're appending two dependency rules. Create the drop-in:

**/etc/systemd/system/docker.service.d/40-flannel.conf***

```yml
[Unit]
Requires=flanneld.service
After=flanneld.service
```

[dropins]: https://coreos.com/os/docs/latest/using-systemd-drop-in-units.html

#### Create the kubelet Unit

The kublet is the agent on each machine that starts and stops Pods, configures iptables rules and other machine-level tasks. The kublet communicates to the API server (also running on the master machine) with the TLS certificates we placed on disk earlier.

* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.
* Replace `${DNS_SERVICE_IP}`

**/etc/systemd/system/kubelet.service**

```yml
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

#### Set Up the kube-apiserver Pod

The API server is where most of the magic happens. It is stateless by design and takes in API requests, processes them and stores the result in etcd if needed, and then returns the result of the request.

We're going to use a unqiue feature of the kublet to launch a Pod runs the API server. Above we configured the kublet to watch a local directory for pods to run with the `--config=/etc/kubernetes/manifests` flag. All we need to do is place our Pod manifest in that location, and the kublet will make sure it stays running, just as if the Pod was submitted via the API. The cool trick here is that we don't have an API running yet, but the Pod will function the exact same way, which simplifies troubleshooting later on.

If this is your first time looking at a Pod manifest, don't worry, they aren't all this complicated. But, this shows off the power and flexibility of the Pod concept. Create `/etc/kubernetes/manifests/kube-apiserver.yaml` and replace the settings:

* Replace `${ETCD_ENDPOINTS}`
* Replace `${SERVICE_IP_RANGE}`
* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.

**/etc/kubernetes/manifests/kube-apiserver.yaml**

```yml
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

#### Set Up the kube-proxy Pod

We're going to run the proxy just like we did the API server. The proxy is responsible for directing traffic destined for specific services and pods to the correct location. The proxy communicates with the API server periodically to keep up to date.

Both the Master and Workers in your cluster will run the proxy.

All you have to do is create `/etc/kubernetes/manifests/kube-proxy.yaml`, there are no settings that need to be configured.

**/etc/kubernetes/manifests/kube-proxy.yaml**

```yml
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

#### Set Up the kube-controller-manager Pod

The controller manager is responsible for reconciling any required actions based on changes to [Replication Controllers][rc-overview]. 

For example, if you increased the replica count, the controller manager would generate a scale up event, which would cause a new Pod to get scheduled in the cluster. The controller manager communicates with the API to submit these events.

Create `/etc/kubernetes/manifests/kube-controller-manager.yaml`. It will use the TLS certificate placed on disk earlier.

[rc-overview]: https://coreos.com/kubernetes/docs/latest/replication-controller.html

**/etc/kubernetes/manifests/kube-controller-manager.yaml**

```yml
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

#### Set Up the kube-scheduler Pod

The scheduler is the last major piece of our Master. It monitors the API for unscheduled pods, finds them a machine to run on, and communicates the decision back to the API.

Create File `/etc/kubernetes/manifests/kube-scheduler.yaml`:

**/etc/kubernetes/manifests/kube-scheduler.yaml**

```yml
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

Now that we've defined all of our units and written our TLS certificates to disk, we're ready to start the Master components.

#### Load Changed Units

First, we need to tell systemd that we've changed units on disk and it needs to rescan everything:

```
$ sudo systemctl daemon-reload
```

#### Configure Flannel Network

Earlier it was mentioned that Flannel stores cluster-level configuration in etcd. We need to configure our Pod network IP range now. Since etcd was started earlier, we can set this now. If you don't have etcd running, start it now.

* Replace `$POD_NETWORK`
* Replace `$ETCD_SERVER` with one host from `$ETCD_ENDPOINTS`

```sh
$ curl -X PUT -d "value={\"Network\":\"$POD_NETWORK\"}" "$ETCD_SERVER/v2/keys/coreos.com/network/config"
```

#### Start Kubelet

Now that everything is configured, we can start the Kubelet, which will also start the Pod manifests for the API server, the controller manager, proxy and scheduler.

```sh
$ sudo systemctl start kubelet
```

Ensure that the kublet will start after a reboot:

```sh
$ sudo systemctl enable kubelet
```

Our Pods should now we starting up and downloading their containers. To check the progress, you can run `docker ps`.

While you're waiting for things to download, let's start deploying our Worker machines.

## Deploy Worker Node(s)

Boot one or more CoreOS nodes which will be used as Kubernetes Workers.

See the [CoreOS Documentation](https://coreos.com/os/docs/latest/) for guides on launching nodes on supported platforms.

### Configure Service Components

#### TLS Assets

Place the TLS keypairs generated previously in the following locations:

* File: `/etc/kubernetes/ssl/ca.pem`
* File: `/etc/kubernetes/ssl/worker.pem`
* File: `/etc/kubernetes/ssl/worker-key.pem`

#### Flannel Configuration

Just like earlier, create `/run/flannel/options.env` and replace your values:


* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

**/run/flannel/options.env**

```yml
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```

#### Docker Configuration

Require that flanneld is running prior to Docker start.

Create `/etc/systemd/system/docker.service.d/40-flannel.conf`

**/etc/systemd/system/docker.service.d/40-flannel.conf**

```yml
[Unit]
Requires=flanneld.service
After=flanneld.service
```

#### Create the kubelet Unit

Create `/etc/systemd/system/kubelet.service` and replace: 

* Replace `${MASTER_IP}`
* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.
* Replace `${DNS_SERVICE_IP}`

**/etc/systemd/system/kubelet.service**

```yml
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

#### Set Up the kube-proxy Pod

Create `/etc/kubernetes/manifests/kube-proxy.yaml` and replace:

* Replace `${MASTER_IP}`

**/etc/kubernetes/manifests/kube-proxy.yaml**

```yml
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

#### Set Up kubeconfig

In order to facilitate secure communication between multiple Kubernetes clusters, kubeconfig can be used to track settings related to each cluster. A user can switch between them when using `kubectl`.

Create `/etc/kubernetes/worker-kubeconfig.yaml`:

**/etc/kubernetes/worker-kubeconfig.yaml**

```yml
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

Now we can start the Worker services.

#### Load Changed Units

Tell systemd to rescan the units on disk:

```sh
$ sudo systemctl daemon-reload
```

#### Start Kubelet

Start the Kublet, which will start the proxy as well.

```sh
$ sudo systemctl start kubelet
```

Ensure that the kublet starts on each boot:

```sh
$ sudo systemctl enable kubelet
```

## Configure kubectl

The primary CLI tool used to interact with the Kubernetes API is called `kubectl`.

The following steps should be done from your local workstation to configure `kubectl` to work with your new cluster.

First, download the binary using a command-line tool such as `wget` or `curl`:

```sh
# Replace ${ARCH} with "linux" or "darwin" based on your workstation operating system
$ wget https://storage.googleapis.com/kubernetes-release/release/v1.0.3/bin/${ARCH}/amd64/kubectl
```

After downloading the binary, ensure it is executable and move it into your PATH:

```sh
$ chmod +x kubectl
$ mv kubectl /usr/local/bin/kubectl
```

Configure your local Kubernetes client using the following commands:

* Replace `${MASTER_IP}`
* Replace `${CA_CERT}` with the path to the `ca.pem` created in previous steps
* Replace `${ADMIN_KEY}` with the path to the `admin-key.pem` created in previous steps
* Replace `${ADMIN_CERT}` with the path to the `admin.pem` created in previous steps

```sh
$ kubectl config set-cluster vagrant --server=${MASTER_IP} --certificate-authority=${CA_CERT}
$ kubectl config set-credentials vagrant-admin --certificate-authority=${CA_CERT} --client-key=${ADMIN_KEY} --client-certificate=${ADMIN_CERT}
$ kubectl config set-context vagrant --cluster=vagrant --user=vagrant-admin
$ kubectl config use-context vagrant
```

Check that your client is configured properly by using `kubectl` to inspect your cluster:

```sh
$ kubectl get nodes
NAME          LABELS                               STATUS
X.X.X.X       kubernetes.io/hostname=X.X.X.X       Ready
```

## Deploy the DNS Add-on

The DNS add-on allows for your services to have a DNS name in addition to an IP address. This is helpful for older pieces of software that can't be reconfigured easily or aren't "Kubernetes-aware".

Add-ons are built on the same Kubernetes components as user-submitted jobs &mdash; Pods, Replication Controllers and Services. We're going to install the DNS add-on with `kubectl`.

First create `dns-addon.yml` on your local machine and replace the variable. There is a lot going on in there, so let's break it down after you create it.

* Replace `${DNS_SERVICE_IP}`

**dns-addon.yml**

```yml
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

This single YAML file is actually creating 3 different Kubernetes objects, separated by `---`. The first is a new namespace called `kube-system`. Namespaces allow you to group related objects together. This namespace is a Kubernetes convention and it holds all add-ons.

The second object is a service that provides DNS lookups over port 53 for any service that requires it.

The third object is a Replication Controller, which consists of several different containers that work together to provide DNS lookups. Thre's too much going on to explain it all, but it's using health checks, resource limits, and intra-pod networking over multiple ports. These are features that you can't get in other container orchestration systems and shows the production knowledge that Google has applied to Kubernetes.

Next, start the DNS add-on:

```sh
$ kubectl create -f dns-addon.yml
```

## Next Steps: Deploy an Application

Now that you have a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.

Start with a [multi-tier web application](http://kubernetes.io/v1.0/examples/guestbook-go/README.html) from the official Kubernetes documentation to visualize how the various Kubernetes components fit together.