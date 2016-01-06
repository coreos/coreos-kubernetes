package cluster

var baseControllerCloudConfig = `#cloud-config
coreos:
  update:
    reboot-strategy: "off"

  flannel:
    interface: $private_ipv4

  etcd2:
    name: controller
    advertise-client-urls: http://$private_ipv4:2379
    initial-advertise-peer-urls: http://$private_ipv4:2380
    listen-client-urls: http://0.0.0.0:2379
    listen-peer-urls: http://0.0.0.0:2380
    initial-cluster: controller=http://$private_ipv4:2380

  units:
  - name: etcd2.service
    command: start

  - name: install-controller.service
    command: start
    content: |
      [Service]
      ExecStartPre=/usr/bin/tar -xf /tmp/cluster-manifests.tar -C /tmp
      ExecStartPre=/usr/bin/tar -xf /tmp/controller-manifests.tar -C /tmp
      ExecStart=/bin/bash /tmp/install-controller.sh
      Type=oneshot

write_files:
- path: /run/coreos-kubernetes/options.env
  content: |
    ETCD_ENDPOINTS=http://127.0.0.1:2379
    SERVICE_IP_RANGE={{ ServiceCIDR }}
    POD_NETWORK={{ PodCIDR }}
    K8S_SERVICE_IP={{ KubernetesServiceIP }}
    DNS_SERVICE_IP={{ DNSServiceIP }}

- path: /tmp/install-controller.sh
  encoding: gzip+base64
  content: {{ InstallControllerScript }}

- path: /tmp/cluster-manifests.tar
  encoding: gzip+base64
  content: {{ ClusterManifestsTar }}

- path: /tmp/controller-manifests.tar
  encoding: gzip+base64
  content: {{ ControllerManifestsTar }}

- path: /etc/kubernetes/ssl/ca.pem
  encoding: gzip+base64
  content: {{ CACert }}

- path: /etc/kubernetes/ssl/apiserver.pem
  encoding: gzip+base64
  content: {{ APIServerCert }}

- path: /etc/kubernetes/ssl/apiserver-key.pem
  encoding: gzip+base64
  content: {{ APIServerKey }}
`
