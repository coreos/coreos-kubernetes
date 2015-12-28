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
      ExecStart=/bin/bash /tmp/install-controller.sh
      Type=oneshot

write_files:
- path: /run/coreos-kubernetes/options.env
  content: |
    ETCD_ENDPOINTS=http://127.0.0.1:2379
    ARTIFACT_URL={{ Ref%ArtifactURL }}
    SERVICE_IP_RANGE={{ Ref%ServiceCIDR }}
    POD_NETWORK={{ Ref%PodCIDR }}
    K8S_SERVICE_IP={{ Ref%KubernetesServiceIP }}
    DNS_SERVICE_IP={{ Ref%DNSServiceIP }}

- path: /tmp/install-controller.sh
  content: |
    #!/bin/bash

    exec bash -c "$(curl --fail --silent --show-error --location '{{ Ref%ArtifactURL }}/scripts/install-controller.sh')"

- path: /etc/kubernetes/ssl/ca.pem
  encoding: base64
  content: {{ Ref%CACert }}

- path: /etc/kubernetes/ssl/apiserver.pem
  encoding: base64
  content: {{ Ref%APIServerCert }}

- path: /etc/kubernetes/ssl/apiserver-key.pem
  encoding: base64
  content: {{ Ref%APIServerKey }}
`
