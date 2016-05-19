# Kubelet Wrapper Script

The kubelet is the orchestrator of containers on each host in the Kubernetes cluster — it starts and stops containers, configures pod mounts, and other low-level, essential tasks. In order to accomplish these tasks, the kubelet requires special permissions on the host.

CoreOS recommends running the kubelet using the rkt container engine, because it has the correct set of features to enable these special permissions, while taking advantage of all that container packaging has to offer: image discovery, signing/verification, and simplified management.

CoreOS ships a wrapper script, `/usr/lib/coreos/kubelet-wrapper`, which makes it very easy to run the kubelet under rkt. This script accomplishes two things:

1. Future releases of CoreOS can tweak the system-related parameters of the kubelet, such as mounting in /etc/ssl/certs.
1. Allows user-specified flags and the desired version of the kubelet to be passed to rkt. This gives each cluster admin control to enable newer API features and easily tweak settings, independent of CoreOS releases.

This script is currently shipping in CoreOS 962.0.0+ and will be included in all channels in the near future.

## Using the kubelet-wrapper

An example systemd kubelet.service file which takes advantage of the kubelet-wrapper script:

**/etc/systemd/system/kubelet.service**

```ini
[Service]
Environment=KUBELET_VERSION=v1.2.4_coreos.1
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=http://127.0.0.1:8080 \
  --config=/etc/kubernetes/manifests
```

In the example above we set the `KUBELET_VERSION` and the kubelet-wrapper script takes care of running the correct container image with our desired API server address and manifest location.

## Customizing rkt Options

Passing customized options or flags to rkt can be accomplished with the RKT_OPTS environment variable. Referencing it in a unit file is straightforward.

### Use the host's DNS configuration

Mount the host's `/etc/resolv.conf` file directly into the container in order to inherit DNS settings and allow you to address workers by hostname in addition to an IP address.

```ini
[Service]
Environment="RKT_OPTS=--volume=resolv,kind=host,source=/etc/resolv.conf --mount volume=resolv,target=/etc/resolv.conf"
Environment=KUBELET_VERSION=v1.2.4_coreos.1
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=http://127.0.0.1:8080 \
  --config=/etc/kubernetes/manifests
  ...other flags...
```

### Allow pods to use iSCSI mounts

Pods running in your cluster can reference remote storage volumes located on an iSCSI target.

```ini
[Service]
Environment="RKT_OPTS=--volume iscsiadm,kind=host,source=/usr/sbin/iscsiadm --mount volume=iscsiadm,target=/usr/sbin/iscsiadm"
Environment=KUBELET_VERSION=v1.2.4_coreos.1
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=http://127.0.0.1:8080 \
  --config=/etc/kubernetes/manifests
  ...other flags...
```

### Use the cluster logging add-on

Export the logs collected by the kubelet via a volume mount, so that [other logging applications][addon-logging] can consume it. In addition to setting `RKT_OPTS`, an `ExecStartPre` is included to create the log directory.

```ini
[Service]
Environment="RKT_OPTS=--volume var-log,kind=host,source=/var/log --mount volume=var-log,target=/var/log"
Environment=KUBELET_VERSION=v1.2.4_coreos.1
ExecStartPre=/usr/bin/mkdir -p /var/log/containers
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=http://127.0.0.1:8080 \
  --config=/etc/kubernetes/manifests
  ...other flags...
```

## Manual deployment

If you wish to use the kubelet-wrapper on a CoreOS version prior to 962.0.0, you can manually place the script on the host. Please note that this requires rkt version 0.15.0+.

For example:

- Retrieve a copy of the [kubelet-wrapper script][kubelet-wrapper]
- Place on the host: `/opt/bin/kubelet-wrapper`
- Make the script executable: `chmod +x /opt/bin/kubelet-wrapper`
- Reference from your kubelet service file:

```ini
[Service]
Environment=KUBELET_VERSION=v1.2.4_coreos.1
ExecStart=/opt/bin/kubelet-wrapper \
  --api-servers=http://127.0.0.1:8080 \
  --config=/etc/kubernetes/manifests
```

[#2141]: https://github.com/coreos/rkt/issues/2141
[kubelet-wrapper]: https://github.com/coreos/coreos-overlay/blob/master/app-admin/kubelet-wrapper/files/kubelet-wrapper
[addon-logging]: https://github.com/kubernetes/kubernetes/tree/release-1.2/cluster/addons/fluentd-elasticsearch
