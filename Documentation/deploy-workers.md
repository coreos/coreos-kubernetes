# Deploy Kubernetes Worker Node(s)

Boot one or more CoreOS Container Linux nodes which will be used as Kubernetes Workers. See the [Container Linux Documentation](https://coreos.com/os/docs/latest/) for guides on launching nodes on supported platforms.

## Configure Service Components

### TLS Assets

Place the TLS keypairs generated previously in the following locations. Note that each keypair is unique and should be installed on the worker node it was generated for:

* File: `/etc/kubernetes/ssl/ca.pem`
* File: `/etc/kubernetes/ssl/${WORKER_FQDN}-worker.pem`
* File: `/etc/kubernetes/ssl/${WORKER_FQDN}-worker-key.pem`

And make sure you've set proper permission for private key:

```sh
$ sudo chmod 600 /etc/kubernetes/ssl/*-key.pem
$ sudo chown root:root /etc/kubernetes/ssl/*-key.pem
```

Create symlinks to the worker-specific certificate and key so that the remaining configurations on the workers do not have to be unique per worker.

```sh
$ cd /etc/kubernetes/ssl/
$ sudo ln -s ${WORKER_FQDN}-worker.pem worker.pem
$ sudo ln -s ${WORKER_FQDN}-worker-key.pem worker-key.pem
```


### Networking Configuration

*Note:* If the pod-network is being managed independently of flannel, then the flannel parts of this guide can be skipped. It's recommended that Calico is still used for providing network policy. See [kubernetes networking](kubernetes-networking.md) for more detail.

Just like earlier, create `/etc/flannel/options.env` and modify these values:

* Replace `${ADVERTISE_IP}` with this node's publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

**/etc/flannel/options.env**

```yaml
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```

Next create a [systemd drop-in][dropins], which will use the above configuration when flannel starts

**/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf**

```yaml
[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
```

[dropins]: https://coreos.com/os/docs/latest/using-systemd-drop-in-units.html

### Docker Configuration

*Note:* If the pod-network is being managed independently, this step can be skipped. See [kubernetes networking](kubernetes-networking.md) for more detail.

Require that flanneld is running prior to Docker start.

Create `/etc/systemd/system/docker.service.d/40-flannel.conf`

**/etc/systemd/system/docker.service.d/40-flannel.conf**

```yaml
[Unit]
Requires=flanneld.service
After=flanneld.service
[Service]
EnvironmentFile=/etc/kubernetes/cni/docker_opts_cni.env
```

Create the Docker CNI Options file:

**/etc/kubernetes/cni/docker_opts_cni.env**

```yaml
DOCKER_OPT_BIP=""
DOCKER_OPT_IPMASQ=""
```

If using Flannel for networking, setup the Flannel CNI configuration with below. If you intend to use Calico for networking, setup using [Set Up the CNI config (optional)](#set-up-the-cni-config-optional) instead.

**/etc/kubernetes/cni/net.d/10-flannel.conf**

```yaml
{
    "name": "podnet",
    "type": "flannel",
    "delegate": {
        "isDefaultGateway": true
    }
}
```

### Create the kubelet Unit

Create `/etc/systemd/system/kubelet.service` and substitute the following variables:

* Replace `${MASTER_HOST}` 
* Replace `${ADVERTISE_IP}` with this node's publicly routable IP.
* Replace `${DNS_SERVICE_IP}`
* Replace `${K8S_VER}` This will map to: `quay.io/coreos/hyperkube:${K8S_VER}` release, e.g. `v1.5.4_coreos.0`.
* If using Calico for network policy
  - Replace `${NETWORK_PLUGIN}` with `cni`
  - Add the following to `RKT_RUN_ARGS=`
    ```
    --volume cni-bin,kind=host,source=/opt/cni/bin \
    --mount volume=cni-bin,target=/opt/cni/bin
    ```
  - Add `ExecStartPre=/usr/bin/mkdir -p /opt/cni/bin`
* Decide if you will use [additional features][rkt-opts-examples] such as:
  - [mounting ephemeral disks][mount-disks]
  - [allow pods to mount RDB][rdb] or [iSCSI volumes][iscsi]
  - [allowing access to insecure container registries][insecure-registry]
  - [changing your Container Linux auto-update settings][update]

**Note**: Anyone with access to port 10250 on a node can execute arbitrary code in a pod on the node. Information, including logs and metadata, is also disclosed on port 10255. See [securing the Kubelet API][securing-kubelet-api] for more information.

**/etc/systemd/system/kubelet.service**

```yaml
[Service]
Environment=KUBELET_IMAGE_TAG=${K8S_VER}
Environment="RKT_RUN_ARGS=--uuid-file-save=/var/run/kubelet-pod.uuid \
  --volume dns,kind=host,source=/etc/resolv.conf \
  --mount volume=dns,target=/etc/resolv.conf \
  --volume var-log,kind=host,source=/var/log \
  --mount volume=var-log,target=/var/log"
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
ExecStartPre=/usr/bin/mkdir -p /var/log/containers
ExecStartPre=-/usr/bin/rkt rm --uuid-file=/var/run/kubelet-pod.uuid
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=https://${MASTER_HOST} \
  --cni-conf-dir=/etc/kubernetes/cni/net.d \
  --network-plugin=${NETWORK_PLUGIN} \
  --container-runtime=docker \
  --register-node=true \
  --allow-privileged=true \
  --pod-manifest-path=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster_dns=${DNS_SERVICE_IP} \
  --cluster_domain=cluster.local \
  --kubeconfig=/etc/kubernetes/worker-kubeconfig.yaml \
  --tls-cert-file=/etc/kubernetes/ssl/worker.pem \
  --tls-private-key-file=/etc/kubernetes/ssl/worker-key.pem
ExecStop=-/usr/bin/rkt stop --uuid-file=/var/run/kubelet-pod.uuid
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Set Up the kube-proxy Pod

Create `/etc/kubernetes/manifests/kube-proxy.yaml`:

* Replace `${MASTER_HOST}`

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
    image: quay.io/coreos/hyperkube:v1.5.4_coreos.0
    command:
    - /hyperkube
    - proxy
    - --master=${MASTER_HOST}
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

### Set Up kubeconfig

In order to facilitate secure communication between Kubernetes components, kubeconfig can be used to define authentication settings. In this case, the kubelet and proxy are reading this configuration to communicate with the API.

Create `/etc/kubernetes/worker-kubeconfig.yaml`:

**/etc/kubernetes/worker-kubeconfig.yaml**

```yaml
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

## Start Services

Now we can start the Worker services.

### Load Changed Units

Tell systemd to rescan the units on disk:

```sh
$ sudo systemctl daemon-reload
```

### Start kubelet, and flannel

Start the kubelet, which will start the proxy.

```sh
$ sudo systemctl start flanneld
$ sudo systemctl start kubelet
```

Ensure that the services start on each boot:

```sh
$ sudo systemctl enable flanneld
Created symlink from /etc/systemd/system/multi-user.target.wants/flanneld.service to /etc/systemd/system/flanneld.service.
$ sudo systemctl enable kubelet
Created symlink from /etc/systemd/system/multi-user.target.wants/kubelet.service to /etc/systemd/system/kubelet.service.
```

To check the health of the kubelet systemd unit that we created, run `systemctl status kubelet.service`.

<div class="co-m-docs-next-step">
  <p><strong>Is the kubelet running?</strong></p>
  <a href="configure-kubectl.md" class="btn btn-primary btn-icon-right"  data-category="Docs Next" data-event="Kubernetes: kubectl">Yes, ready to configure `kubectl`</a>
</div>

[rkt-opts-examples]: kubelet-wrapper.md#customizing-rkt-options
[rdb]: kubelet-wrapper.md#allow-pods-to-use-rbd-volumes
[iscsi]: kubelet-wrapper.md#allow-pods-to-use-iscsi-mounts
[host-dns]: kubelet-wrapper.md#use-the-hosts-dns-configuration
[mount-disks]: https://coreos.com/os/docs/latest/mounting-storage.html
[insecure-registry]: https://coreos.com/os/docs/latest/registry-authentication.html#using-a-registry-without-ssl-configured
[update]: https://coreos.com/os/docs/latest/switching-channels.html
[securing-kubelet-api]: kubelet-wrapper.md#Securing-the-Kubelet-API
