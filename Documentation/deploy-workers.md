# Deploy Kubernetes Worker Node(s)

Boot one or more CoreOS nodes which will be used as Kubernetes Workers. You must use a CoreOS version 962.0.0+ for the `/usr/lib/coreos/kubelet-wrapper` script to be present in the image. See [kubelet-wrapper](kubelet-wrapper.md) for more information.

See the [CoreOS Documentation](https://coreos.com/os/docs/latest/) for guides on launching nodes on supported platforms.

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
```

### Create the kubelet Unit

Create `/etc/systemd/system/kubelet.service` and substitute the following variables:

* Replace `${MASTER_HOST}`
* Replace `${ADVERTISE_IP}` with this node's publicly routable IP.
* Replace `${DNS_SERVICE_IP}`
* Replace `${K8S_VER}` This will map to: `quay.io/coreos/hyperkube:${K8S_VER}` release. If using Calico a version that includes CNI binaries should be used. e.g. `v1.2.4_coreos.cni.0`
* Replace `${NETWORK_PLUGIN}` with `cni` if using Calico. Otherwise just leave it blank.

**/etc/systemd/system/kubelet.service**

```yaml
[Service]
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests

Environment=KUBELET_VERSION=${K8S_VER}
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=https://${MASTER_HOST} \
  --network-plugin-dir=/etc/kubernetes/cni/net.d \
  --network-plugin=${NETWORK_PLUGIN} \
  --register-node=true \
  --allow-privileged=true \
  --config=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster-dns=${DNS_SERVICE_IP} \
  --cluster-domain=cluster.local \
  --kubeconfig=/etc/kubernetes/worker-kubeconfig.yaml \
  --tls-cert-file=/etc/kubernetes/ssl/worker.pem \
  --tls-private-key-file=/etc/kubernetes/ssl/worker-key.pem
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
```

### Set Up the CNI config

The kubelet reads the CNI configuration on startup and uses that to determine which CNI plugin to call. Create the following file which tells the kubelet to call the flannel plugin but to then delegate control to the Calico plugin. Using the flannel plugin ensures that the Calico plugin is called with the IP range for the node that was selected by flannel.

Note that this configuration is different to the one on the master nodes. It includes additional Kubernetes authentication information since the API server isn't available on localhost.

* Replace `${ADVERTISE_IP}` with this node's publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`
* Replace `${CONTROLER_IP}`

**/etc/kubernetes/cni/net.d/10-calico.conf**

```json
{
    "name": "calico",
    "type": "flannel",
    "delegate": {
        "type": "calico",
        "etcd_endpoints": "$ETCD_ENDPOINTS",
        "log_level": "none",
        "log_level_stderr": "info",
        "hostname": "${ADVERTISE_IP}",
        "policy": {
            "type": "k8s",
            "k8s_api_root": "https://${CONTROLER_IP}:443/api/v1/",
            "k8s_client_key": "/etc/kubernetes/ssl/worker-key.pem",
            "k8s_client_certificate": "/etc/kubernetes/ssl/worker.pem"
        }
    }
}
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
    image: quay.io/coreos/hyperkube:v1.2.4_coreos.0
    command:
    - /hyperkube
    - proxy
    - --master=https://${MASTER_HOST}
    - --kubeconfig=/etc/kubernetes/worker-kubeconfig.yaml
    - --proxy-mode=iptables
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

### Set Up Calico Node Container

The Calico node container runs on all hosts, including the master node. It performs two functions:
* Connects containers to the flannel overlay network, which enables the "one IP per pod" concept.
* Enforces network policy created through the Kubernetes policy API, ensuring pods talk to authorized resources only.

This step can be skipped if not using Calico.

Create `/etc/systemd/system/calico-node.service` and substitute the following variables:

* Replace `${ADVERTISE_IP}` with this node's publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

**/etc/systemd/system/calico-node.service**

```yaml
[Unit]
Description=Calico node for network policy
Requires=network-online.target
After=network-online.target

[Service]
Slice=machine.slice
Environment=CALICO_DISABLE_FILE_LOGGING=true
Environment=HOSTNAME=${ADVERTISE_IP}
Environment=IP=${ADVERTISE_IP}
Environment=FELIX_FELIXHOSTNAME=${ADVERTISE_IP}
Environment=CALICO_NETWORKING=false
Environment=NO_DEFAULT_POOLS=true
Environment=ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
ExecStart=/usr/bin/rkt run --inherit-env --stage1-from-dir=stage1-fly.aci \
--volume=modules,kind=host,source=/lib/modules,readOnly=false \
--mount=volume=modules,target=/lib/modules \
--trust-keys-from-https quay.io/calico/node:v0.19.0
KillMode=mixed
Restart=always
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
```

## Start Services

Now we can start the Worker services.

### Load Changed Units

Tell systemd to rescan the units on disk:

```sh
$ sudo systemctl daemon-reload
```

### Start kubelet, flannel and Calico Node

Start the kubelet, which will start the proxy as well as the Calico node (if required).

```sh
$ sudo systemctl start flanneld
$ sudo systemctl start kubelet
$ sudo systemctl start calico-node
```

Ensure that the services start on each boot:

```sh
$ sudo systemctl enable flanneld
Created symlink from /etc/systemd/system/multi-user.target.wants/flanneld.service to /etc/systemd/system/flanneld.service.
$ sudo systemctl enable kubelet
Created symlink from /etc/systemd/system/multi-user.target.wants/kubelet.service to /etc/systemd/system/kubelet.service.
$ sudo systemctl enable calico-node
Created symlink from /etc/systemd/system/multi-user.target.wants/calico-node.service to /etc/systemd/system/calico-node.service.
```

To check the health of the kubelet systemd unit that we created, run `systemctl status kubelet.service`.
To check the health of the calico-node systemd unit that we created, run `systemctl status calico-node.service`.

<div class="co-m-docs-next-step">
  <p><strong>Is the kubelet running?</strong></p>
  <a href="configure-kubectl.md" class="btn btn-primary btn-icon-right"  data-category="Docs Next" data-event="Kubernetes: kubectl">Yes, ready to configure `kubectl`</a>
</div>
