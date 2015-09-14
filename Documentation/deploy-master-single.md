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

[Flannel][flannel-docs] provides a key Kubernetes networking capability &mdash; a software-defined overlay network to manage routing of the [Pod][pod-overview] network.

Flannel stores local configuration in `/run/flannel/options.env` and cluster-level configuration in etcd. Create this file and edit the contents:

* Replace `${ADVERTISE_IP}` with this machine's publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

**/run/flannel/options.env**

```sh
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```

[flannel-docs]: https://coreos.com/flannel/docs/latest/
[pod-overview]: https://coreos.com/kubernetes/docs/latest/pods.html
[service-overview]: https://coreos.com/kubernetes/docs/latest/services.html

#### Docker Configuration

In order for Flannel to manage the pod network in the cluster, Docker needs to be configured to use it. All we need to do is require that flanneld is running prior to Docker starting.

We're going to do this with a [systemd drop-in][dropins], which is a method for appending or overriding parameters of a systemd unit. In this case we're appending two dependency rules. Create the drop-in:

**/etc/systemd/system/docker.service.d/40-flannel.conf***

```yaml
[Unit]
Requires=flanneld.service
After=flanneld.service
```

[dropins]: https://coreos.com/os/docs/latest/using-systemd-drop-in-units.html

#### Create the kubelet Unit

The kublet is the agent on each machine that starts and stops Pods and other machine-level tasks. The kublet communicates to the API server (also running on the master machine) with the TLS certificates we placed on disk earlier.

* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.
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

We're going to use a unqiue feature of the kublet to launch a Pod that runs the API server. Above we configured the kublet to watch a local directory for pods to run with the `--config=/etc/kubernetes/manifests` flag. All we need to do is place our Pod manifest in that location, and the kublet will make sure it stays running, just as if the Pod was submitted via the API. The cool trick here is that we don't have an API running yet, but the Pod will function the exact same way, which simplifies troubleshooting later on.

If this is your first time looking at a Pod manifest, don't worry, they aren't all this complicated. But, this shows off the power and flexibility of the Pod concept. Create `/etc/kubernetes/manifests/kube-apiserver.yaml` and replace the settings:

* Replace `${ETCD_ENDPOINTS}`
* Replace `${SERVICE_IP_RANGE}`
* Replace `${ADVERTISE_IP}` with this nodes publicly routable IP.

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

```yaml
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

Our Pods should now starting up and downloading their containers. To check the download progress, you can run `docker ps`.

To check the health of the Kubelet systemd unit that we created, run `systemctl status kubelet.service`.

If you run into issues with Docker and Flannel, check to see that the drop-in was applied correctly by running `systemctl cat docker.service` and ensuring that the drop-in appears at the bottom.

<div class="co-m-docs-next-step">
  <p><strong>Did the containers start downloading?</strong> As long as they started to download, everything is working properly.</p>
  <a href="deploy-workers.md" class="btn btn-primary btn-icon-right">Yes, ready to deploy the Workers</a>
</div>
