#!/bin/bash

if [ -d certs ]; then
  echo "There is already a certs directory."
  exit 0
fi

echo "Generating TLS assets..."
mkdir -p certs
../../lib/init-ssl-ca ./certs

# the apiserver can be addressed locally, over the NAT bridge, or by internal cluster IP
../../lib/init-ssl ./certs apiserver kube-apiserver IP.1=127.0.0.1,IP.2=192.168.120.10,IP.3=10.3.0.1
../../lib/init-ssl ./certs worker kube-worker IP.1=127.0.0.1,IP.2=192.168.120.11
../../lib/init-ssl ./certs admin kube-admin
