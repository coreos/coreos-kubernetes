# Deploy Kubernetes Master Machine

Boot a single CoreOS machine which will be used as the Kubernetes master. You must use a CoreOS version 773.1.0+ on the Alpha or Beta channel for the `kubelet` to be present in the image.

See the [CoreOS Documentation](https://coreos.com/os/docs/latest/) for guides on launching nodes on supported platforms.

Manual configuration of the required Master services is explained below, but most of the configuration could also be done with cloud-config, aside from placing the TLS assets on disk. These secrets shouldn't be stored in cloud-config for enhanced security.

If you are deploying multiple master nodes in a high-availability cluster, these instructions can be repeated for each master node you wish to launch.

## Configure Service Components

### TLS Assets

Place the keys generated previously in the following locations:

* File: `/etc/kubernetes/ssl/ca.pem`
* File: `/etc/kubernetes/ssl/apiserver.pem`
* File: `/etc/kubernetes/ssl/apiserver-key.pem`

And make sure you've set proper permission for private key:

```
$ sudo chmod 600 /etc/kubernetes/ssl/*-key.pem
$ sudo chown root:root /etc/kubernetes/ssl/*-key.pem
```

### flannel Configuration

[flannel][flannel-docs] provides a key Kubernetes networking capability &mdash; a software-defined overlay network to manage routing of the [Pod][pod-overview] network.

*Note:* If the pod-network is being managed independently of flannel, this step can be skipped. See [kubernetes networking](kubernetes-networking.md) for more detail.

We will configure flannel to source its local configuration in `/etc/flannel/options.env` and cluster-level configuration in etcd. Create this file and edit the contents:

* Replace `${ADVERTISE_IP}` with this machine's publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

**/etc/flannel/options.env**

```sh
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```

Next create a [systemd drop-in][dropins], which will use the above configuration when flannel starts

**/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf**

```yaml
[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
```

[flannel-docs]: https://coreos.com/flannel/docs/latest/
[pod-overview]: https://coreos.com/kubernetes/docs/latest/pods.html
[service-overview]: https://coreos.com/kubernetes/docs/latest/services.html

### Docker Configuration

In order for flannel to manage the pod network in the cluster, Docker needs to be configured to use it. All we need to do is require that flanneld is running prior to Docker starting.

*Note:* If the pod-network is being managed independently of flannel, this step can be skipped. See [kubernetes networking](kubernetes-networking.md) for more detail.

We're going to do this with a [systemd drop-in][dropins], which is a method for appending or overriding parameters of a systemd unit. In this case we're appending two dependency rules. Create the drop-in:

**/etc/systemd/system/docker.service.d/40-flannel.conf**

```yaml
[Unit]
Requires=flanneld.service
After=flanneld.service
```

[dropins]: https://coreos.com/os/docs/latest/using-systemd-drop-in-units.html

### Create the kubelet Unit

The [kubelet](http://kubernetes.io/v1.1/docs/admin/kubelet.html) is the agent on each machine that starts and stops Pods and other machine-level tasks. The kubelet communicates with the API server (also running on the master machines) with the TLS certificates we placed on disk earlier.

On the master node, the kubelet is configured to communicate with the API server, but not register for cluster work, as shown in the `--register-node=false` line in the YAML excerpt below. This prevents user pods being scheduled on the master nodes, and ensures cluster work is routed only to task-specific worker nodes.

Note that the kubelet running on a master node may log repeated attempts to post its status to the API server. These warnings are expected behavior and can be ignored. Future Kubernetes releases plan to [handle this common deployment consideration more gracefully](https://github.com/kubernetes/kubernetes/issues/14140#issuecomment-142126864).

* Replace `${ADVERTISE_IP}` with this node's publicly routable IP.
* Replace `${DNS_SERVICE_IP}`

**/etc/systemd/system/kubelet.service**

```yaml
[Service]
ExecStart=/usr/bin/kubelet \
  --api_servers=http://127.0.0.1:8080 \
  --register-node=false \
  --allow-privileged=true \
  --config=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster-dns=${DNS_SERVICE_IP} \
  --cluster-domain=cluster.local
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
```

### Set Up the kube-apiserver Pod

The API server is where most of the magic happens. It is stateless by design and takes in API requests, processes them and stores the result in etcd if needed, and then returns the result of the request.

We're going to use a unique feature of the kubelet to launch a Pod that runs the API server. Above we configured the kubelet to watch a local directory for pods to run with the `--config=/etc/kubernetes/manifests` flag. All we need to do is place our Pod manifest in that location, and the kubelet will make sure it stays running, just as if the Pod was submitted via the API. The cool trick here is that we don't have an API running yet, but the Pod will function the exact same way, which simplifies troubleshooting later on.

If this is your first time looking at a Pod manifest, don't worry, they aren't all this complicated. But, this shows off the power and flexibility of the Pod concept. Create `/etc/kubernetes/manifests/kube-apiserver.yaml` with the following settings:

* Replace `${ETCD_ENDPOINTS}`
* Replace `${SERVICE_IP_RANGE}`
* Replace `${ADVERTISE_IP}` with this node's publicly routable IP.

**/etc/kubernetes/manifests/kube-apiserver.yaml**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-apiserver
    image: gcr.io/google_containers/hyperkube:v1.1.2
    command:
    - /hyperkube
    - apiserver
    - --bind-address=0.0.0.0
    - --etcd-servers=${ETCD_ENDPOINTS}
    - --allow-privileged=true
    - --service-cluster-ip-range=${SERVICE_IP_RANGE}
    - --secure-port=443
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
    - containerPort: 8080
      hostPort: 8080
      name: local
    volumeMounts:
    - mountPath: /etc/kubernetes/ssl
      name: ssl-certs-kubernetes
      readOnly: true
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
  volumes:
  - hostPath:
      path: /etc/kubernetes/ssl
    name: ssl-certs-kubernetes
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
```

### Set Up the kube-proxy Pod

We're going to run the proxy just like we did the API server. The proxy is responsible for directing traffic destined for specific services and pods to the correct location. The proxy communicates with the API server periodically to keep up to date.

Both the Master and Workers in your cluster will run the proxy.

All you have to do is create `/etc/kubernetes/manifests/kube-proxy.yaml`, there are no settings that need to be configured.

**/etc/kubernetes/manifests/kube-proxy.yaml**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-proxy
    image: gcr.io/google_containers/hyperkube:v1.1.2
    command:
    - /hyperkube
    - proxy
    - --master=http://127.0.0.1:8080
    - --proxy-mode=iptables
    securityContext:
      privileged: true
    volumeMounts:
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
  volumes:
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
```

### Set up the kube-podmaster Pod

The kube-podmaster is responsible for implementing master-election for the kube-controller-manager and kube-scheduler. Because these services modify the cluster state, we only want to have one actor making modifications at a time.

In a single-master deployment, the kube-podmaster will simply run the kube-scheduler and kube-controller-manager on the current node. In a multi-master deployment, the kube-podmaster will be responsible for starting a new instance of the Kubernetes components in the case of a machine dying.

When creating `/etc/kubernetes/manifests/kube-podmaster.yaml`:

* Replace `${ETCD_ENDPOINTS}`
* Replace `${ADVERTISE_IP}`

**/etc/kubernetes/manifests/kube-podmaster.yaml**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kube-podmaster
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: scheduler-elector
    image: gcr.io/google_containers/podmaster:1.1
    command:
    - /podmaster
    - --etcd-servers=${ETCD_ENDPOINTS}
    - --key=scheduler
    - --whoami=${ADVERTISE_IP}
    - --source-file=/src/manifests/kube-scheduler.yaml
    - --dest-file=/dst/manifests/kube-scheduler.yaml
    volumeMounts:
    - mountPath: /src/manifests
      name: manifest-src
      readOnly: true
    - mountPath: /dst/manifests
      name: manifest-dst
  - name: controller-manager-elector
    image: gcr.io/google_containers/podmaster:1.1
    command:
    - /podmaster
    - --etcd-servers=${ETCD_ENDPOINTS}
    - --key=controller
    - --whoami=${ADVERTISE_IP}
    - --source-file=/src/manifests/kube-controller-manager.yaml
    - --dest-file=/dst/manifests/kube-controller-manager.yaml
    terminationMessagePath: /dev/termination-log
    volumeMounts:
    - mountPath: /src/manifests
      name: manifest-src
      readOnly: true
    - mountPath: /dst/manifests
      name: manifest-dst
  volumes:
  - hostPath:
      path: /srv/kubernetes/manifests
    name: manifest-src
  - hostPath:
      path: /etc/kubernetes/manifests
    name: manifest-dst
```

### Set Up the kube-controller-manager Pod

The controller manager is responsible for reconciling any required actions based on changes to [Replication Controllers][rc-overview].

For example, if you increased the replica count, the controller manager would generate a scale up event, which would cause a new Pod to get scheduled in the cluster. The controller manager communicates with the API to submit these events.

Create `/srv/kubernetes/manifests/kube-controller-manager.yaml`. It will use the TLS certificate placed on disk earlier.

[rc-overview]: https://coreos.com/kubernetes/docs/latest/replication-controller.html

**/srv/kubernetes/manifests/kube-controller-manager.yaml**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-controller-manager
    image: gcr.io/google_containers/hyperkube:v1.1.2
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
      name: ssl-certs-kubernetes
      readOnly: true
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
  volumes:
  - hostPath:
      path: /etc/kubernetes/ssl
    name: ssl-certs-kubernetes
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
```

### Set Up the kube-scheduler Pod

The scheduler is the last major piece of our Master. It monitors the API for unscheduled pods, finds them a machine to run on, and communicates the decision back to the API.

Create File `/srv/kubernetes/manifests/kube-scheduler.yaml`:

**/srv/kubernetes/manifests/kube-scheduler.yaml**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-scheduler
    image: gcr.io/google_containers/hyperkube:v1.1.2
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

## Start Services

Now that we've defined all of our units and written our TLS certificates to disk, we're ready to start the Master components.

### Load Changed Units

First, we need to tell systemd that we've changed units on disk and it needs to rescan everything:

```
$ sudo systemctl daemon-reload
```

### Configure flannel Network

Earlier it was mentioned that flannel stores cluster-level configuration in etcd. We need to configure our Pod network IP range now. Since etcd was started earlier, we can set this now. If you don't have etcd running, start it now.

* Replace `$POD_NETWORK`
* Replace `$ETCD_SERVER` with one url (`http://ip:port`) from `$ETCD_ENDPOINTS`

```sh
$ curl -X PUT -d "value={\"Network\":\"$POD_NETWORK\",\"Backend\":{\"Type\":\"vxlan\"}}" "$ETCD_SERVER/v2/keys/coreos.com/network/config"
```

### Start kubelet

Now that everything is configured, we can start the kubelet, which will also start the Pod manifests for the API server, the controller manager, proxy and scheduler.

```sh
$ sudo systemctl start kubelet
```

Ensure that the kubelet will start after a reboot:

```sh
$ sudo systemctl enable kubelet
Created symlink from /etc/systemd/system/multi-user.target.wants/kubelet.service to /etc/systemd/system/kubelet.service.
```

### Create kube-system Namespace

The Kubernetes Pods that make up the Master node will exist in their own namespace. We need to create this namespace so these components are discoverable by other nodes in the cluster.

**Note**: You will only need to do this once per-cluster. If deploying multiple master nodes, this step needs to happen only once.

First, we need to make sure the Kubernetes API is available (this could take a few minutes after starting the kubelet.service)

```
curl http://127.0.0.1:8080/version
```

A successful response should look something like:

```
{
  "major": "1",
  "minor": "0",
  "gitVersion": "v1.1.2",
  "gitCommit": "388061f00f0d9e4d641f9ed4971c775e1654579d",
  "gitTreeState": "clean"
}
```

Now we can create the `kube-system` namespace:

```
curl -XPOST -d'{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"kube-system"}}' "http://127.0.0.1:8080/api/v1/namespaces"
```

Our Pods should now be starting up and downloading their containers. To check the download progress, you can run `docker ps`.

To check the health of the kubelet systemd unit that we created, run `systemctl status kubelet.service`.

If you run into issues with Docker and flannel, check to see that the drop-in was applied correctly by running `systemctl cat docker.service` and ensuring that the drop-in appears at the bottom.

<div class="co-m-docs-next-step">
  <p><strong>Did the containers start downloading?</strong> As long as they started to download, everything is working properly.</p>
  <a href="deploy-workers.md" class="btn btn-primary btn-icon-right" data-category="Docs Next" data-event="Kubernetes: Workers">Yes, ready to deploy the Workers</a>
</div>
