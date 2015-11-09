#!/bin/bash

if [ -d certs ]; then
  echo "There is already a certs directory."
  exit 0
fi

echo "Generating TLS assets..."
mkdir -p certs
../../lib/init-ssl-ca ./certs
../../lib/init-ssl ./certs apiserver kube-apiserver IP.1=127.0.0.1,IP.2=192.168.120.10
../../lib/init-ssl ./certs worker kube-worker IP.1=127.0.0.1,IP.2=192.168.120.11
../../lib/init-ssl ./certs admin kube-admin
