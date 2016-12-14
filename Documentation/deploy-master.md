# Deploy Kubernetes Master Node(s)

Boot a single CoreOS machine which will be used as the Kubernetes master node. You must use a CoreOS version 962.0.0+ for the `/usr/lib/coreos/kubelet-wrapper` script to be present in the image. See [kubelet-wrapper](kubelet-wrapper.md) for more information.

See the [CoreOS Documentation](https://coreos.com/os/docs/latest/) for guides on launching nodes on supported platforms.

Manual configuration of the required master node services is explained below, but most of the configuration could also be done with cloud-config, aside from placing the TLS assets on disk. For security reasons, these secrets should not be stored in cloud-config.

The instructions below configure the required master node components using manifests stored in `/etc/kubernetes/manifests`. The kubelet will watch this location for new or modified manifests and run them automatically.

High-availability is achieved by repeating these instructions for each master node. Each of the master components is safe to run on multiple nodes.

The apiserver is stateless, but handles recording the results of leader elections to etcd on behalf of other master components. The controller-manager and scheduler use the leader election mechanism to ensure only one of each is active, leaving the inactive master components ready to assume responsibility in case of failure.

## Configure Service Components

### TLS Assets

Create the required directory and place the keys generated previously in the following locations:

```sh
$ sudo mkdir -p /etc/kubernetes/ssl
```

* File: `/etc/kubernetes/ssl/ca.pem`
* File: `/etc/kubernetes/ssl/apiserver.pem`
* File: `/etc/kubernetes/ssl/apiserver-key.pem`

And make sure you've set proper permission for private key:

```sh
$ sudo chmod 600 /etc/kubernetes/ssl/*-key.pem
$ sudo chown root:root /etc/kubernetes/ssl/*-key.pem
```

### Network Configuration

Networking is provided by Flannel and Calico.

* [flannel][flannel-docs] provides a software-defined overlay network for routing traffic to/from the [pods][pod-overview]
* [Calico][calico-docs] secures the overlay network by restricting traffic to/from the pods based on fine-grained network policy.

*Note:* If the pod-network is being managed independently of flannel, then the flannel parts of this guide can be skipped. In this case, Calico may still be used for providing network policy. See [Kubernetes networking](kubernetes-networking.md) for more detail.

We will configure flannel to source its local configuration in `/etc/flannel/options.env` and cluster-level configuration in etcd. Create this file and edit the contents:

* Replace `${ADVERTISE_IP}` with this machine's publicly routable IP.
* Replace `${ETCD_ENDPOINTS}`

**/etc/flannel/options.env**

```sh
FLANNELD_IFACE=${ADVERTISE_IP}
FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
```
Next create a [systemd drop-in][dropins], which is a method for appending or overriding parameters of a systemd unit. In this case we're appending two dependency rules. Create the following drop-in, which will use the above configuration when flannel starts:

**/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf**

```yaml
[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
```

[calico-docs]: http://docs.projectcalico.org/v2.0/getting-started/kubernetes/
[flannel-docs]: https://coreos.com/flannel/docs/latest/
[pod-overview]: https://coreos.com/kubernetes/docs/latest/pods.html
[service-overview]: https://coreos.com/kubernetes/docs/latest/services.html

### Docker Configuration

In order for flannel to manage the pod network in the cluster, Docker needs to be configured to use it. All we need to do is require that flanneld is running prior to Docker starting.

*Note:* If the pod-network is being managed independently, this step can be skipped. See [kubernetes networking](kubernetes-networking.md) for more detail.

Again, we will use a [systemd drop-in][dropins]:

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

[dropins]: https://coreos.com/os/docs/latest/using-systemd-drop-in-units.html

### Create the kubelet Unit

The [kubelet](http://kubernetes.io/docs/admin/kubelet.html) is the agent on each machine that starts and stops Pods and other machine-level tasks. The kubelet communicates with the API server (also running on the master nodes) with the TLS certificates we placed on disk earlier.

On the master node, the kubelet is configured to communicate with the API server, but not register for cluster work, as shown in the `--register-schedulable=false` line in the YAML excerpt below. This prevents user pods being scheduled on the master nodes, and ensures cluster work is routed only to task-specific worker nodes.

When using Calico, the kubelet is configured to use the Container Networking Interface (CNI) standard for networking. This makes Calico aware of each pod that is created and allows it to network the pods into the flannel overlay. Both flannel and Calico communicate via CNI interfaces to ensure the correct IP range (managed by flannel) is used for each node.

Note that the kubelet running on a master node may log repeated attempts to post its status to the API server. These warnings are expected behavior and can be ignored. Future Kubernetes releases plan to [handle this common deployment consideration more gracefully](https://github.com/kubernetes/kubernetes/issues/14140#issuecomment-142126864).

* Replace `${ADVERTISE_IP}` with this node's publicly routable IP.
* Replace `${DNS_SERVICE_IP}`
* Replace `${K8S_VER}` This will map to: `quay.io/coreos/hyperkube:${K8S_VER}` release, e.g. `v1.5.1_coreos.0`.
* If using Calico for network policy
  - Replace `${NETWORK_PLUGIN}` with `cni`
  - Add the following to `RKT_OPS=`
    ```
    --volume cni-bin,kind=host,source=/opt/cni/bin \
    --mount volume=cni-bin,target=/opt/cni/bin
    ```
  - Add `ExecStartPre=/usr/bin/mkdir -p /opt/cni/bin`
* Decide if you will use [additional features][rkt-opts-examples] such as:
  - [mounting ephemeral disks][mount-disks]
  - [allow pods to mount RDB][rdb] or [iSCSI volumes][iscsi]
  - [allowing access to insecure container registries][insecure-registry]
  - [changing your CoreOS auto-update settings][update]

**/etc/systemd/system/kubelet.service**

```yaml
[Service]
Environment=KUBELET_VERSION=${K8S_VER}
Environment="RKT_OPTS=--uuid-file-save=/var/run/kubelet-pod.uuid \
  --volume var-log,kind=host,source=/var/log \
  --mount volume=var-log,target=/var/log
  --volume dns,kind=host,source=/etc/resolv.conf \
  --mount volume=dns,target=/etc/resolv.conf"
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
ExecStartPre=/usr/bin/mkdir -p /var/log/containers
ExecStartPre=-/usr/bin/rkt rm --uuid-file=/var/run/kubelet-pod.uuid
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=http://127.0.0.1:8080 \
  --register-schedulable=false \
  --cni-conf-dir=/etc/kubernetes/cni/net.d \
  --network-plugin=cni \
  --container-runtime=docker \
  --allow-privileged=true \
  --pod-manifest-path=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster_dns=${DNS_SERVICE_IP} \
  --cluster_domain=cluster.local
ExecStop=-/usr/bin/rkt stop --uuid-file=/var/run/kubelet-pod.uuid
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Set Up the kube-apiserver Pod

The API server is where most of the magic happens. It is stateless by design and takes in API requests, processes them and stores the result in etcd if needed, and then returns the result of the request.

We're going to use a unique feature of the kubelet to launch a Pod that runs the API server. Above we configured the kubelet to watch a local directory for pods to run with the `--pod-manifest-path=/etc/kubernetes/manifests` flag. All we need to do is place our Pod manifest in that location, and the kubelet will make sure it stays running, just as if the Pod was submitted via the API. The cool trick here is that we don't have an API running yet, but the Pod will function the exact same way, which simplifies troubleshooting later on.

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
    image: quay.io/coreos/hyperkube:v1.5.1_coreos.0
    command:
    - /hyperkube
    - apiserver
    - --bind-address=0.0.0.0
    - --etcd-servers=${ETCD_ENDPOINTS}
    - --allow-privileged=true
    - --service-cluster-ip-range=${SERVICE_IP_RANGE}
    - --secure-port=443
    - --advertise-address=${ADVERTISE_IP}
    - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota
    - --tls-cert-file=/etc/kubernetes/ssl/apiserver.pem
    - --tls-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
    - --client-ca-file=/etc/kubernetes/ssl/ca.pem
    - --service-account-key-file=/etc/kubernetes/ssl/apiserver-key.pem
    - --runtime-config=extensions/v1beta1/networkpolicies=true
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        port: 8080
        path: /healthz
      initialDelaySeconds: 15
      timeoutSeconds: 15
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

Both the master and worker nodes in your cluster will run the proxy.

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
    image: quay.io/coreos/hyperkube:v1.5.1_coreos.0
    command:
    - /hyperkube
    - proxy
    - --master=http://127.0.0.1:8080
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

### Set Up the kube-controller-manager Pod

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
  hostNetwork: true
  containers:
  - name: kube-controller-manager
    image: quay.io/coreos/hyperkube:v1.5.1_coreos.0
    command:
    - /hyperkube
    - controller-manager
    - --master=http://127.0.0.1:8080
    - --leader-elect=true
    - --service-account-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
    - --root-ca-file=/etc/kubernetes/ssl/ca.pem
    resources:
      requests:
        cpu: 200m
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10252
      initialDelaySeconds: 15
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /etc/kubernetes/ssl
      name: ssl-certs-kubernetes
      readOnly: true
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
  hostNetwork: true
  volumes:
  - hostPath:
      path: /etc/kubernetes/ssl
    name: ssl-certs-kubernetes
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
```

### Set Up the kube-scheduler Pod

The scheduler monitors the API for unscheduled pods, finds them a machine to run on, and communicates the decision back to the API.

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
    image: quay.io/coreos/hyperkube:v1.5.1_coreos.0
    command:
    - /hyperkube
    - scheduler
    - --master=http://127.0.0.1:8080
    - --leader-elect=true
    resources:
      requests:
        cpu: 100m
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10251
      initialDelaySeconds: 15
      timeoutSeconds: 15
```

### Set Up Calico For Network Policy (optional)

This step can be skipped if you do not wish to provide network policy to your cluster using Calico.

Several things happen here.  First the `ConfigMap` will create some configuration needed for Calico and the kubelet.

Second the `DaemonSet` runs on all hosts, including the master node. It performs several functions:
* The `install-cni` container drops in the necessary CNI binaries to `/opt/cni/bin/` that was setup in the kubelet step
* Connects containers to the flannel overlay network, which enables the "one IP per pod" concept.
* Enforces network policy created through the Kubernetes policy API, ensuring pods talk to authorized resources only.

The policy controller is the last major piece of the calico.yaml. It monitors the API for changes related to network policy and configures Calico to implement that policy. 

When creating `/etc/kubernetes/manifests/calico.yaml`:

* Replace `${ETCD_ENDPOINTS}`

**/etc/kubernetes/manifests/calico.yaml**

```yaml
# This ConfigMap is used to configure a self-hosted Calico installation.
kind: ConfigMap
apiVersion: v1
metadata:
  name: calico-config 
  namespace: kube-system
data:
  # Configure this with the location of your etcd cluster.
  etcd_endpoints: "${ETCD_ENDPOINTS}"

  # The CNI network configuration to install on each node.  The special
  # values in this config will be automatically populated.
  cni_network_config: |-
    {
        "name": "calico",
        "type": "flannel",
        "delegate": {
          "type": "calico",
          "etcd_endpoints": "__ETCD_ENDPOINTS__",
          "log_level": "info",
          "policy": {
              "type": "k8s",
              "k8s_api_root": "https://__KUBERNETES_SERVICE_HOST__:__KUBERNETES_SERVICE_PORT__",
              "k8s_auth_token": "__SERVICEACCOUNT_TOKEN__"
          },
          "kubernetes": {
              "kubeconfig": "/etc/kubernetes/cni/net.d/__KUBECONFIG_FILENAME__"
          }
        }
    }

---

# This manifest installs the calico/node container, as well
# as the Calico CNI plugins and network config on 
# each master and worker node in a Kubernetes cluster.
kind: DaemonSet
apiVersion: extensions/v1beta1
metadata:
  name: calico-node
  namespace: kube-system
  labels:
    k8s-app: calico-node
spec:
  selector:
    matchLabels:
      k8s-app: calico-node
  template:
    metadata:
      labels:
        k8s-app: calico-node
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
        scheduler.alpha.kubernetes.io/tolerations: |
          [{"key": "dedicated", "value": "master", "effect": "NoSchedule" },
           {"key":"CriticalAddonsOnly", "operator":"Exists"}]
    spec:
      hostNetwork: true
      containers:
        # Runs calico/node container on each Kubernetes node.  This 
        # container programs network policy and routes on each
        # host.
        - name: calico-node
          image: quay.io/calico/node:v0.23.0
          env:
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # Choose the backend to use. 
            - name: CALICO_NETWORKING_BACKEND
              value: "none"
            # Disable file logging so `kubectl logs` works.
            - name: CALICO_DISABLE_FILE_LOGGING
              value: "true"
            - name: NO_DEFAULT_POOLS
              value: "true"
          securityContext:
            privileged: true
          volumeMounts:
            - mountPath: /lib/modules
              name: lib-modules
              readOnly: false
            - mountPath: /var/run/calico
              name: var-run-calico
              readOnly: false
            - mountPath: /etc/resolv.conf
              name: dns
              readOnly: true
        # This container installs the Calico CNI binaries
        # and CNI network config file on each node.
        - name: install-cni
          image: quay.io/calico/cni:v1.5.2
          imagePullPolicy: Always
          command: ["/install-cni.sh"]
          env:
            # CNI configuration filename
            - name: CNI_CONF_NAME
              value: "10-calico.conf"
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # The CNI network config to install on each node.
            - name: CNI_NETWORK_CONFIG
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: cni_network_config
          volumeMounts:
            - mountPath: /host/opt/cni/bin
              name: cni-bin-dir
            - mountPath: /host/etc/cni/net.d
              name: cni-net-dir
      volumes:
        # Used by calico/node.
        - name: lib-modules
          hostPath:
            path: /lib/modules
        - name: var-run-calico
          hostPath:
            path: /var/run/calico
        # Used to install CNI.
        - name: cni-bin-dir
          hostPath:
            path: /opt/cni/bin
        - name: cni-net-dir
          hostPath:
            path: /etc/kubernetes/cni/net.d
        - name: dns
          hostPath:
            path: /etc/resolv.conf

---

# This manifest deploys the Calico policy controller on Kubernetes.
# See https://github.com/projectcalico/k8s-policy
apiVersion: extensions/v1beta1
kind: ReplicaSet 
metadata:
  name: calico-policy-controller
  namespace: kube-system
  labels:
    k8s-app: calico-policy
spec:
  # The policy controller can only have a single active instance.
  replicas: 1
  template:
    metadata:
      name: calico-policy-controller
      namespace: kube-system
      labels:
        k8s-app: calico-policy
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
        scheduler.alpha.kubernetes.io/tolerations: |
          [{"key": "dedicated", "value": "master", "effect": "NoSchedule" },
           {"key":"CriticalAddonsOnly", "operator":"Exists"}]
    spec:
      # The policy controller must run in the host network namespace so that
      # it isn't governed by policy that would prevent it from working.
      hostNetwork: true
      containers:
        - name: calico-policy-controller
          image: calico/kube-policy-controller:v0.4.0
          env:
            # The location of the Calico etcd cluster.
            - name: ETCD_ENDPOINTS
              valueFrom:
                configMapKeyRef:
                  name: calico-config
                  key: etcd_endpoints
            # The location of the Kubernetes API.  Use the default Kubernetes
            # service for API access.
            - name: K8S_API
              value: "https://kubernetes.default:443"
            # Since we're running in the host namespace and might not have KubeDNS 
            # access, configure the container's /etc/hosts to resolve
            # kubernetes.default to the correct service clusterIP.
            - name: CONFIGURE_ETC_HOSTS
              value: "true"
```

## Start Services

Now that we've defined all of our units and written our TLS certificates to disk, we're ready to start the master components.

### Load Changed Units

First, we need to tell systemd that we've changed units on disk and it needs to rescan everything:

```sh
$ sudo systemctl daemon-reload
```

### Configure flannel Network

Earlier it was mentioned that flannel stores cluster-level configuration in etcd. We need to configure our Pod network IP range now. Since etcd was started earlier, we can set this now. If you don't have etcd running, start it now.

* Replace `$POD_NETWORK`
* Replace `$ETCD_SERVER` with one url (`http://ip:port`) from `$ETCD_ENDPOINTS`

```sh
$ curl -X PUT -d "value={\"Network\":\"$POD_NETWORK\",\"Backend\":{\"Type\":\"vxlan\"}}" "$ETCD_SERVER/v2/keys/coreos.com/network/config"
```

After configuring flannel, we should restart it for our changes to take effect. Note that this will also restart the docker daemon and could impact running containers.

```sh
$ sudo systemctl start flanneld
$ sudo systemctl enable flanneld
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

### Create Namespaces

The Kubernetes Pods that make up the control plane will exist in their own namespace. We need to create this namespace so these components are discoverable by other hosts in the cluster.

**Note**: You will only need to do this once per-cluster. If deploying multiple master nodes, this step needs to happen only once.

First, we need to make sure the Kubernetes API is available (this could take a few minutes after starting the kubelet.service)

```sh
$ curl http://127.0.0.1:8080/version
```

A successful response should look something like:

```
{
  "major": "1",
  "minor": "4",
  "gitVersion": "v1.5.1+coreos.0",
  "gitCommit": "ec2b52fabadf824a42b66b6729fe4cff2c62af8c",
  "gitTreeState": "clean",
  "buildDate": "2016-11-14T19:42:00Z",
  "goVersion": "go1.6.3",
  "compiler": "gc",
  "platform": "linux/amd64"
}
```
Now we can create the `kube-system` namespace:

```sh
$ curl -H "Content-Type: application/json" -XPOST -d'{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"kube-system"}}' "http://127.0.0.1:8080/api/v1/namespaces"
```

To check the health of the kubelet systemd unit that we created, run `systemctl status kubelet.service`.

Our Pods should now be starting up and downloading their containers. Once the kubelet has started, you can check it's creating its pods via the metadata api:

```sh
$ curl -s localhost:10255/pods | jq -r '.items[].metadata.name'
kube-scheduler-$node
kube-apiserver-$node
kube-controller-$node
kube-proxy-$node
```

<div class="co-m-docs-next-step">
  <p><strong>Did the containers start downloading?</strong> As long as the kubelet knows about them, everything is working properly.</p>
  <a href="deploy-workers.md" class="btn btn-primary btn-icon-right" data-category="Docs Next" data-event="Kubernetes: Workers">Yes, ready to deploy the Workers</a>
</div>

[rkt-opts-examples]: kubelet-wrapper.md#customizing-rkt-options
[rdb]: kubelet-wrapper.md#allow-pods-to-use-rbd-volumes
[iscsi]: kubelet-wrapper.md#allow-pods-to-use-iscsi-mounts
[host-dns]: kubelet-wrapper.md#use-the-hosts-dns-configuration
[mount-disks]: https://coreos.com/os/docs/latest/mounting-storage.html
[insecure-registry]: https://coreos.com/os/docs/latest/registry-authentication.html#using-a-registry-without-ssl-configured
[update]: https://coreos.com/os/docs/latest/switching-channels.html
