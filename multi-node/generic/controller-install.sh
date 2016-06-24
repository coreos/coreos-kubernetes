#!/bin/bash
set -e

# List of etcd servers (http://ip:port), comma separated
export ETCD_ENDPOINTS=

# Specify the version (vX.Y.Z) of Kubernetes assets to deploy
export K8S_VER=v1.3.0-beta.2_coreos.0

# Hyperkube image repository to use.
export HYPERKUBE_IMAGE_REPO=quay.io/coreos/hyperkube

# The CIDR network to use for pod IPs.
# Each pod launched in the cluster will be assigned an IP out of this range.
# Each node will be configured such that these IPs will be routable using the flannel overlay network.
export POD_NETWORK=10.2.0.0/16

# The CIDR network to use for service cluster IPs.
# Each service will be assigned a cluster IP out of this range.
# This must not overlap with any IP ranges assigned to the POD_NETWORK, or other existing network infrastructure.
# Routing to these IPs is handled by a proxy service local to each node, and are not required to be routable between nodes.
export SERVICE_IP_RANGE=10.3.0.0/24

# The IP address of the Kubernetes API Service
# If the SERVICE_IP_RANGE is changed above, this must be set to the first IP in that range.
export K8S_SERVICE_IP=10.3.0.1

# The IP address of the cluster DNS service.
# This IP must be in the range of the SERVICE_IP_RANGE and cannot be the first IP in the range.
# This same IP must be configured on all worker nodes to enable DNS service discovery.
export DNS_SERVICE_IP=10.3.0.10

# Whether to use Calico for Kubernetes network policy.
export USE_CALICO=false

# The above settings can optionally be overridden using an environment file:
ENV_FILE=/run/coreos-kubernetes/options.env

# -------------

function init_config {
    local REQUIRED=('ADVERTISE_IP' 'POD_NETWORK' 'ETCD_ENDPOINTS' 'SERVICE_IP_RANGE' 'K8S_SERVICE_IP' 'DNS_SERVICE_IP' 'K8S_VER' 'HYPERKUBE_IMAGE_REPO' 'USE_CALICO')

    if [ -f $ENV_FILE ]; then
        export $(cat $ENV_FILE | xargs)
    fi

    if [ -z $ADVERTISE_IP ]; then
        export ADVERTISE_IP=$(awk -F= '/COREOS_PUBLIC_IPV4/ {print $2}' /etc/environment)
    fi

    for REQ in "${REQUIRED[@]}"; do
        if [ -z "$(eval echo \$$REQ)" ]; then
            echo "Missing required config value: ${REQ}"
            exit 1
        fi
    done

    if [ $USE_CALICO = "true" ]; then
        export K8S_NETWORK_PLUGIN="cni"
    else
        export K8S_NETWORK_PLUGIN=""
    fi
}

function init_flannel {
    echo "Waiting for etcd..."
    while true
    do
        IFS=',' read -ra ES <<< "$ETCD_ENDPOINTS"
        for ETCD in "${ES[@]}"; do
            echo "Trying: $ETCD"
            if [ -n "$(curl --silent "$ETCD/v2/machines")" ]; then
                local ACTIVE_ETCD=$ETCD
                break
            fi
            sleep 1
        done
        if [ -n "$ACTIVE_ETCD" ]; then
            break
        fi
    done
    RES=$(curl --silent -X PUT -d "value={\"Network\":\"$POD_NETWORK\",\"Backend\":{\"Type\":\"vxlan\"}}" "$ACTIVE_ETCD/v2/keys/coreos.com/network/config?prevExist=false")
    if [ -z "$(echo $RES | grep '"action":"create"')" ] && [ -z "$(echo $RES | grep 'Key already exists')" ]; then
        echo "Unexpected error configuring flannel pod network: $RES"
    fi
}

function init_templates {
    local TEMPLATE=/etc/systemd/system/kubelet.service
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
Environment=KUBELET_VERSION=${K8S_VER}
Environment=KUBELET_ACI=${HYPERKUBE_IMAGE_REPO}
ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
ExecStart=/usr/lib/coreos/kubelet-wrapper \
  --api-servers=http://127.0.0.1:8080 \
  --register-schedulable=false \
  --network-plugin-dir=/etc/kubernetes/cni/net.d \
  --network-plugin=${K8S_NETWORK_PLUGIN} \
  --allow-privileged=true \
  --config=/etc/kubernetes/manifests \
  --hostname-override=${ADVERTISE_IP} \
  --cluster_dns=${DNS_SERVICE_IP} \
  --cluster_domain=cluster.local
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    }


    local TEMPLATE=/etc/systemd/system/calico-node.service
    [ "$USE_CALICO" = "true" ] && [ ! -f $TEMPLATE ] && {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Unit]
Description=Calico per-host agent
Requires=network-online.target
After=network-online.target

[Service]
Slice=machine.slice
Environment=CALICO_DISABLE_FILE_LOGGING=true
Environment=HOSTNAME=${ADVERTISE_IP}
Environment=IP=${ADVERTISE_IP}
Environment=FELIX_FELIXHOSTNAME=${ADVERTISE_IP}
Environment=CALICO_NETWORKING=false
Environment=NO_DEFAULT_POOLS=true
Environment=ETCD_ENDPOINTS=${ETCD_ENDPOINTS}
ExecStart=/usr/bin/rkt run --inherit-env --stage1-from-dir=stage1-fly.aci \
--volume=modules,kind=host,source=/lib/modules,readOnly=false \
--mount=volume=modules,target=/lib/modules \
--trust-keys-from-https quay.io/calico/node:v0.19.0
KillMode=mixed
Restart=always
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF
    }


    local TEMPLATE=/etc/kubernetes/manifests/kube-proxy.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-proxy
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
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
EOF
    }

    local TEMPLATE=/etc/kubernetes/manifests/kube-apiserver.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-apiserver
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
    command:
    - /hyperkube
    - apiserver
    - --bind-address=0.0.0.0
    - --etcd-servers=${ETCD_ENDPOINTS}
    - --allow-privileged=true
    - --service-cluster-ip-range=${SERVICE_IP_RANGE}
    - --secure-port=443
    - --advertise-address=${ADVERTISE_IP}
    - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,ResourceQuota
    - --tls-cert-file=/etc/kubernetes/ssl/apiserver.pem
    - --tls-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
    - --client-ca-file=/etc/kubernetes/ssl/ca.pem
    - --service-account-key-file=/etc/kubernetes/ssl/apiserver-key.pem
    - --runtime-config=extensions/v1beta1/thirdpartyresources=true
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
EOF
    }

    local TEMPLATE=/etc/kubernetes/manifests/kube-controller-manager.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
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
EOF
    }

    local TEMPLATE=/etc/kubernetes/manifests/kube-scheduler.yaml
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-scheduler
    image: ${HYPERKUBE_IMAGE_REPO}:$K8S_VER
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
EOF
    }

    local TEMPLATE=/etc/kubernetes/manifests/calico-policy-agent.yaml
    [ "$USE_CALICO" = "true" ] && [ ! -f $TEMPLATE ] && {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
apiVersion: v1
kind: Pod
metadata:
  name: calico-policy-agent
  namespace: calico-system
spec:
  hostNetwork: true
  containers:
    # The Calico policy agent.
    - name: k8s-policy-agent
      image: calico/k8s-policy-agent:v0.1.4
      env:
        - name: ETCD_ENDPOINTS
          value: "${ETCD_ENDPOINTS}"
        - name: K8S_API
          value: "http://127.0.0.1:8080"
        - name: LEADER_ELECTION
          value: "true"
    # Leader election container used by the policy agent.
    - name: leader-elector
      image: quay.io/calico/leader-elector:v0.1.0
      imagePullPolicy: IfNotPresent
      args:
        - "--election=calico-policy-election"
        - "--election-namespace=calico-system"
        - "--http=127.0.0.1:4040"
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/calico-system.json
    [ "$USE_CALICO" = "true" ] && [ ! -f $TEMPLATE ] && {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
  "apiVersion": "v1",
  "kind": "Namespace",
  "metadata": {
    "name": "calico-system"
  }
}
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/network-policy.json
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
  "kind": "ThirdPartyResource",
  "apiVersion": "extensions/v1beta1",
  "metadata": {
    "name": "network-policy.net.alpha.kubernetes.io"
  },
  "description": "Specification for a network isolation policy",
  "versions": [
    {
      "name": "v1alpha1"
    }
  ]
}
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/kube-dns-rc.json
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
  "apiVersion": "v1",
  "kind": "ReplicationController",
  "metadata": {
    "labels": {
      "k8s-app": "kube-dns",
      "kubernetes.io/cluster-service": "true",
      "version": "v15"
    },
    "name": "kube-dns-v15",
    "namespace": "kube-system"
  },
  "spec": {
    "replicas": 1,
    "selector": {
      "k8s-app": "kube-dns",
      "version": "v15"
    },
    "template": {
      "metadata": {
        "labels": {
          "k8s-app": "kube-dns",
          "kubernetes.io/cluster-service": "true",
          "version": "v15"
        }
      },
      "spec": {
        "containers": [
          {
            "args": [
              "--domain=cluster.local.",
              "--dns-port=10053"
            ],
            "image": "gcr.io/google_containers/kubedns-amd64:1.3",
            "livenessProbe": {
              "failureThreshold": 5,
              "httpGet": {
                "path": "/healthz",
                "port": 8080,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 60,
              "successThreshold": 1,
              "timeoutSeconds": 5
            },
            "name": "kubedns",
            "ports": [
              {
                "containerPort": 10053,
                "name": "dns-local",
                "protocol": "UDP"
              },
              {
                "containerPort": 10053,
                "name": "dns-tcp-local",
                "protocol": "TCP"
              }
            ],
            "readinessProbe": {
              "httpGet": {
                "path": "/readiness",
                "port": 8081,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 30,
              "timeoutSeconds": 5
            },
            "resources": {
              "limits": {
                "cpu": "100m",
                "memory": "200Mi"
              },
              "requests": {
                "cpu": "100m",
                "memory": "50Mi"
              }
            }
          },
          {
            "args": [
              "--cache-size=1000",
              "--no-resolv",
              "--server=127.0.0.1#10053"
            ],
            "image": "gcr.io/google_containers/kube-dnsmasq-amd64:1.3",
            "name": "dnsmasq",
            "ports": [
              {
                "containerPort": 53,
                "name": "dns",
                "protocol": "UDP"
              },
              {
                "containerPort": 53,
                "name": "dns-tcp",
                "protocol": "TCP"
              }
            ]
          },
          {
            "args": [
              "-cmd=nslookup kubernetes.default.svc.cluster.local 127.0.0.1 >/dev/null",
              "-port=8080",
              "-quiet"
            ],
            "image": "gcr.io/google_containers/exechealthz-amd64:1.0",
            "name": "healthz",
            "ports": [
              {
                "containerPort": 8080,
                "protocol": "TCP"
              }
            ],
            "resources": {
              "limits": {
                "cpu": "10m",
                "memory": "20Mi"
              },
              "requests": {
                "cpu": "10m",
                "memory": "20Mi"
              }
            }
          }
        ],
        "dnsPolicy": "Default"
      }
    }
  }
}
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/kube-dns-svc.json
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "labels": {
      "k8s-app": "kube-dns",
      "kubernetes.io/cluster-service": "true",
      "kubernetes.io/name": "KubeDNS"
    },
    "name": "kube-dns",
    "namespace": "kube-system"
  },
  "spec": {
    "clusterIP": "$DNS_SERVICE_IP",
    "ports": [
      {
        "name": "dns",
        "port": 53,
        "protocol": "UDP"
      },
      {
        "name": "dns-tcp",
        "port": 53,
        "protocol": "TCP"
      }
    ],
    "selector": {
      "k8s-app": "kube-dns"
    }
  }
}
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/heapster-de.json
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
  "apiVersion": "extensions/v1beta1",
  "kind": "Deployment",
  "metadata": {
    "labels": {
      "k8s-app": "heapster",
      "kubernetes.io/cluster-service": "true",
      "version": "v1.1.0"
    },
    "name": "heapster-v1.1.0",
    "namespace": "kube-system"
  },
  "spec": {
    "replicas": 1,
    "selector": {
      "matchLabels": {
        "k8s-app": "heapster",
        "version": "v1.1.0"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "k8s-app": "heapster",
          "version": "v1.1.0"
        }
      },
      "spec": {
        "containers": [
          {
            "command": [
              "/heapster",
              "--source=kubernetes.summary_api:''"
            ],
            "image": "gcr.io/google_containers/heapster:v1.1.0",
            "name": "heapster",
            "resources": {
              "limits": {
                "cpu": "100m",
                "memory": "200Mi"
              },
              "requests": {
                "cpu": "100m",
                "memory": "200Mi"
              }
            }
          },
          {
            "command": [
              "/pod_nanny",
              "--cpu=100m",
              "--extra-cpu=0.5m",
              "--memory=200Mi",
              "--extra-memory=4Mi",
              "--threshold=5",
              "--deployment=heapster-v1.1.0",
              "--container=heapster",
              "--poll-period=300000",
              "--estimator=exponential"
            ],
            "env": [
              {
                "name": "MY_POD_NAME",
                "valueFrom": {
                  "fieldRef": {
                    "fieldPath": "metadata.name"
                  }
                }
              },
              {
                "name": "MY_POD_NAMESPACE",
                "valueFrom": {
                  "fieldRef": {
                    "fieldPath": "metadata.namespace"
                  }
                }
              }
            ],
            "image": "gcr.io/google_containers/addon-resizer:1.3",
            "name": "heapster-nanny",
            "resources": {
              "limits": {
                "cpu": "50m",
                "memory": "100Mi"
              },
              "requests": {
                "cpu": "50m",
                "memory": "100Mi"
              }
            }
          }
        ]
      }
    }
  }
}
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/heapster-svc.json
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "labels": {
      "kubernetes.io/cluster-service": "true",
      "kubernetes.io/name": "Heapster"
    },
    "name": "heapster",
    "namespace": "kube-system"
  },
  "spec": {
    "ports": [
      {
        "port": 80,
        "targetPort": 8082
      }
    ],
    "selector": {
      "k8s-app": "heapster"
    }
  }
}
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/kube-dashboard-rc.json
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
  "apiVersion": "v1",
  "kind": "ReplicationController",
  "metadata": {
    "labels": {
      "k8s-app": "kubernetes-dashboard",
      "kubernetes.io/cluster-service": "true",
      "version": "v1.1.0-beta3"
    },
    "name": "kubernetes-dashboard-v1.1.0-beta3",
    "namespace": "kube-system"
  },
  "spec": {
    "replicas": 1,
    "selector": {
      "k8s-app": "kubernetes-dashboard"
    },
    "template": {
      "metadata": {
        "labels": {
          "k8s-app": "kubernetes-dashboard",
          "kubernetes.io/cluster-service": "true",
          "version": "v1.1.0-beta3"
        }
      },
      "spec": {
        "containers": [
          {
            "image": "gcr.io/google_containers/kubernetes-dashboard-amd64:v1.1.0-beta3",
            "livenessProbe": {
              "httpGet": {
                "path": "/",
                "port": 9090
              },
              "initialDelaySeconds": 30,
              "timeoutSeconds": 30
            },
            "name": "kubernetes-dashboard",
            "ports": [
              {
                "containerPort": 9090
              }
            ],
            "resources": {
              "limits": {
                "cpu": "100m",
                "memory": "50Mi"
              },
              "requests": {
                "cpu": "100m",
                "memory": "50Mi"
              }
            }
          }
        ]
      }
    }
  }
}
EOF
    }

    local TEMPLATE=/srv/kubernetes/manifests/kube-dashboard-svc.json
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "labels": {
      "kubernetes.io/cluster-service": "true",
      "kubernetes.io/name": "Heapster"
    },
    "name": "heapster",
    "namespace": "kube-system"
  },
  "spec": {
    "ports": [
      {
        "port": 80,
        "targetPort": 8082
      }
    ],
    "selector": {
      "k8s-app": "heapster"
    }
  }
}
EOF
    }

    local TEMPLATE=/etc/flannel/options.env
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
FLANNELD_IFACE=$ADVERTISE_IP
FLANNELD_ETCD_ENDPOINTS=$ETCD_ENDPOINTS
EOF
    }

    local TEMPLATE=/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf.conf
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
EOF
    }

    local TEMPLATE=/etc/systemd/system/docker.service.d/40-flannel.conf
    [ -f $TEMPLATE ] || {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
[Unit]
Requires=flanneld.service
After=flanneld.service
EOF
    }

    local TEMPLATE=/etc/kubernetes/cni/net.d/10-calico.conf
    [ "$USE_CALICO" = "true" ] && [ ! -f $TEMPLATE ] && {
        echo "TEMPLATE: $TEMPLATE"
        mkdir -p $(dirname $TEMPLATE)
        cat << EOF > $TEMPLATE
{
    "name": "calico",
    "type": "flannel",
    "delegate": {
        "type": "calico",
        "etcd_endpoints": "$ETCD_ENDPOINTS",
        "log_level": "none",
        "log_level_stderr": "info",
        "hostname": "${ADVERTISE_IP}",
        "policy": {
            "type": "k8s",
            "k8s_api_root": "http://127.0.0.1:8080/api/v1/"
        }
    }
}
EOF
    }

}

function start_addons {
    echo "Waiting for Kubernetes API..."
    until curl --silent "http://127.0.0.1:8080/version"
    do
        sleep 5
    done
    echo
    echo "K8S: DNS addon"
    curl --silent -H "Content-Type: application/json" -XPOST -d"$(cat /srv/kubernetes/manifests/kube-dns-rc.json)" "http://127.0.0.1:8080/api/v1/namespaces/kube-system/replicationcontrollers" > /dev/null
    curl --silent -H "Content-Type: application/json" -XPOST -d"$(cat /srv/kubernetes/manifests/kube-dns-svc.json)" "http://127.0.0.1:8080/api/v1/namespaces/kube-system/services" > /dev/null
    echo "K8S: Heapster addon"
    curl --silent -H "Content-Type: application/json" -XPOST -d"$(cat /srv/kubernetes/manifests/heapster-de.json)" "http://127.0.0.1:8080/apis/extensions/v1beta1/namespaces/kube-system/deployments" > /dev/null
    curl --silent -H "Content-Type: application/json" -XPOST -d"$(cat /srv/kubernetes/manifests/heapster-svc.json)" "http://127.0.0.1:8080/api/v1/namespaces/kube-system/services" > /dev/null
    echo "K8S: Dashboard addon"
    curl --silent -H "Content-Type: application/json" -XPOST -d"$(cat /srv/kubernetes/manifests/kube-dashboard-rc.json)" "http://127.0.0.1:8080/apis/extensions/v1beta1/namespaces/kube-system/deployments" > /dev/null
    curl --silent -H "Content-Type: application/json" -XPOST -d"$(cat /srv/kubernetes/manifests/kube-dashbaord-svc.json)" "http://127.0.0.1:8080/api/v1/namespaces/kube-system/services" > /dev/null
}

function enable_calico_policy {
    echo "Waiting for Kubernetes API..."
    until curl --silent "http://127.0.0.1:8080/version"
    do
        sleep 5
    done
    echo
    echo "K8S: Calico Policy"
    curl --silent -H "Content-Type: application/json" -XPOST -d"$(cat /srv/kubernetes/manifests/calico-system.json)" "http://127.0.0.1:8080/api/v1/namespaces/" > /dev/null
    curl --silent -H "Content-Type: application/json" -XPOST -d"$(cat /srv/kubernetes/manifests/network-policy.json)" "http://127.0.0.1:8080/apis/extensions/v1beta1/namespaces/default/thirdpartyresources" >/dev/null
}

init_config
init_templates

init_flannel

systemctl stop update-engine; systemctl mask update-engine

systemctl daemon-reload
systemctl enable flanneld; systemctl start flanneld
systemctl enable kubelet; systemctl start kubelet
if [ $USE_CALICO = "true" ]; then
        systemctl enable calico-node; systemctl start calico-node
        enable_calico_policy
fi

start_addons
echo "DONE"
