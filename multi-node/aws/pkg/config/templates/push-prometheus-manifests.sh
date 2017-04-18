#!/bin/bash

set -euo pipefail

# grafana-deployment.json
curl --insecure \
     -d@"./grafana-deployment.json" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer ${BRIDGE_K8S_BEARER_TOKEN}" \
     https://scratchpad.culturematic.net/apis/extensions/v1beta1/namespaces/prometheus/deployments

# grafana-service.json
curl --insecure \
     -d@"./grafana-service.json" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer ${BRIDGE_K8S_BEARER_TOKEN}" \
     https://scratchpad.culturematic.net/api/v1/namespaces/prometheus/services

# node-exporter-ds.json
curl --insecure \
     -d@"./node-exporter-ds.json" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer ${BRIDGE_K8S_BEARER_TOKEN}" \
     https://scratchpad.culturematic.net/apis/extensions/v1beta1/namespaces/prometheus/daemonsets

# node-exporter-service.json
curl --insecure \
     -d@"./node-exporter-service.json" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer ${BRIDGE_K8S_BEARER_TOKEN}" \
     https://scratchpad.culturematic.net/api/v1/namespaces/prometheus/services

# prometheus-configmap.json
curl --insecure \
     -d@"./prometheus-configmap.json" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer ${BRIDGE_K8S_BEARER_TOKEN}" \
     https://scratchpad.culturematic.net/api/v1/namespaces/prometheus/configmaps

# prometheus-deployment.json
curl --insecure \
     -d@"./prometheus-deployment.json" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer ${BRIDGE_K8S_BEARER_TOKEN}" \
     https://scratchpad.culturematic.net/apis/extensions/v1beta1/namespaces/prometheus/deployments

# prometheus-node-exporter-ds.json
curl --insecure \
     -d@"./prometheus-node-exporter-ds.json" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer ${BRIDGE_K8S_BEARER_TOKEN}" \
     https://scratchpad.culturematic.net/apis/extensions/v1beta1/namespaces/prometheus/daemonsets

# prometheus-service.json
curl --insecure \
     -d@"./prometheus-service.json" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer ${BRIDGE_K8S_BEARER_TOKEN}" \
     https://scratchpad.culturematic.net/api/v1/namespaces/prometheus/services
