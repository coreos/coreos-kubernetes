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

```yaml
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```

#### Docker Configuration

Require that flanneld is running prior to Docker start.

Create `/etc/systemd/system/docker.service.d/40-flannel.conf`

**/etc/systemd/system/docker.service.d/40-flannel.conf**

```yaml
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

```yaml
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

In order to facilitate secure communication between Kubernetes components, kubeconfig can be used to define authentication settings. In this case, the kublet and proxy are reading this configuration to communicate with the API.

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
Created symlink from /etc/systemd/system/multi-user.target.wants/kubelet.service to /etc/systemd/system/kubelet.service.
```

To check the health of the Kubelet systemd unit that we created, run `systemctl status kubelet.service`.

If you run into issues with Docker and Flannel, check to see that the drop-in was applied correctly by running `systemctl cat docker.service` and ensuring that the drop-in appears at the bottom.

<div class="co-m-docs-next-step">
  <p><strong>Is the Kubelet running?</strong></p>
  <a href="configure-kubectl.md" class="btn btn-primary btn-icon-right"  data-category="Docs Next" data-event="Kubernetes: kubectl">Yes, ready to configure `kubectl`</a>
</div>
