package cluster

var baseWorkerCloudConfig = `#cloud-config
coreos:
  update:
    reboot-strategy: "off"

  flannel:
    interface: $private_ipv4
    etcd_endpoints: http://{{ GetAtt%LoadBalancerController%DNSName }}:2379

  units:
  - name: install-worker.service
    command: start
    content: |
      [Service]
      ExecStart=/bin/bash /tmp/install-worker.sh
      Type=oneshot

write_files:
- path: /run/coreos-kubernetes/options.env
  content: |
    ETCD_ENDPOINTS=http://{{ GetAtt%LoadBalancerController%DNSName }}:2379
    CONTROLLER_ENDPOINT=https://{{ GetAtt%LoadBalancerController%DNSName }}
    ARTIFACT_URL={{ Ref%ArtifactURL }}
    DNS_SERVICE_IP={{ Ref%DNSServiceIP }}
- path: /tmp/install-worker.sh
  content: |
    #!/bin/bash

    exec bash -c "$(curl --fail --silent --show-error --location '{{ Ref%ArtifactURL }}/scripts/install-worker.sh')"

- path: /etc/kubernetes/ssl/ca.pem
  encoding: base64
  content: {{ Ref%CACert }}

- path: /etc/kubernetes/ssl/worker.pem
  encoding: base64
  content: {{ Ref%WorkerCert }}

- path: /etc/kubernetes/ssl/worker-key.pem
  encoding: base64
  content: {{ Ref%WorkerKey }}
`
