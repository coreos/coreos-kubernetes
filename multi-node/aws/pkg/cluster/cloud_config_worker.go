package cluster

var baseWorkerCloudConfig = `#cloud-config
coreos:
  update:
    reboot-strategy: "off"

  flannel:
    interface: $private_ipv4
    etcd_endpoints: http://{{ ControllerIP }}:2379

  units:
  - name: install-worker.service
    command: start
    content: |
      [Service]
      ExecStartPre=/usr/bin/tar -xf /tmp/cluster-manifests.tar -C /tmp
      ExecStartPre=/usr/bin/tar -xf /tmp/worker-manifests.tar -C /tmp
      ExecStart=/bin/bash /tmp/install-worker.sh
      Type=oneshot

write_files:
- path: /run/coreos-kubernetes/options.env
  content: |
    ETCD_ENDPOINTS=http://{{ ControllerIP }}:2379
    CONTROLLER_ENDPOINT=https://{{ ControllerIP }}
    DNS_SERVICE_IP={{ DNSServiceIP }}

- path: /tmp/install-worker.sh
  encoding: gzip+base64
  content: {{ InstallWorkerScript }}

- path: /tmp/cluster-manifests.tar
  encoding: gzip+base64
  content: {{ ClusterManifestsTar }}

- path: /tmp/worker-manifests.tar
  encoding: gzip+base64
  content: {{ WorkerManifestsTar }}

- path: /etc/kubernetes/ssl/ca.pem
  encoding: gzip+base64
  content: {{ CACert }}

- path: /etc/kubernetes/ssl/worker.pem
  encoding: gzip+base64
  content: {{ WorkerCert }}

- path: /etc/kubernetes/ssl/worker-key.pem
  encoding: gzip+base64
  content: {{ WorkerKey }}
`
