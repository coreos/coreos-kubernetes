## Deploy the DNS Add-on

The DNS add-on allows your services to have a DNS name in addition to an IP address. This is helpful for simplified service discovery between applications. More info can be found in the [Kubernetes DNS documentation][k8s-dns].

Add-ons are built on the same Kubernetes components as user-submitted jobs &mdash; Pods, Replication Controllers and Services. We're going to install the DNS add-on with `kubectl`.

First create `dns-addon.yml` on your local machine and replace the variable. There is a lot going on in there, so let's break it down after you create it.

[k8s-dns]: http://kubernetes.io/docs/admin/dns.html

* Replace `${DNS_SERVICE_IP}`

**dns-addon.yml**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "KubeDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: ${DNS_SERVICE_IP}
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP


---


apiVersion: v1
kind: ReplicationController
metadata:
  name: kube-dns-v17.1
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    version: v17.1
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 1
  selector:
    k8s-app: kube-dns
    version: v17.1
  template:
    metadata:
      labels:
        k8s-app: kube-dns
        version: v17.1
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
      - name: kubedns
        image: gcr.io/google_containers/kubedns-amd64:1.5
        resources:
          limits:
            cpu: 100m
            memory: 170Mi
          requests:
            cpu: 100m
            memory: 70Mi
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
        readinessProbe:
          httpGet:
            path: /readiness
            port: 8081
            scheme: HTTP
          # we poll on pod startup for the Kubernetes master service and
          # only setup the /readiness HTTP server once that's available.
          initialDelaySeconds: 30
          timeoutSeconds: 5
        args:
        # command = "/kube-dns"
        - --domain=cluster.local.
        - --dns-port=10053
        ports:
        - containerPort: 10053
          name: dns-local
          protocol: UDP
        - containerPort: 10053
          name: dns-tcp-local
          protocol: TCP
      - name: dnsmasq
        image: gcr.io/google_containers/kube-dnsmasq-amd64:1.3
        args:
        - --cache-size=1000
        - --no-resolv
        - --server=127.0.0.1#10053
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
      - name: healthz
        image: gcr.io/google_containers/exechealthz-amd64:1.1
        resources:
          # keep request = limit to keep this container in guaranteed class
          limits:
            cpu: 10m
            memory: 50Mi
          requests:
            cpu: 10m
            memory: 50Mi
        args:
        - -cmd=nslookup kubernetes.default.svc.cluster.local 127.0.0.1 >/dev/null && nslookup kubernetes.default.svc.cluster.local 127.0.0.1:10053 >/dev/null
        - -port=8080
        - -quiet
        ports:
        - containerPort: 8080
          protocol: TCP
      dnsPolicy: Default  # Don't use cluster DNS.
```

*Note:* The above YAML definition is based on the upstream DNS addon in the [Kubernetes addon folder][k8s-dns-addon].

[k8s-dns-addon]: https://github.com/kubernetes/kubernetes/tree/v1.3.6/cluster/saltbase/salt/kube-dns

This single YAML file is actually creating 2 different Kubernetes objects, separated by `---`.

The first object is a service that provides DNS lookups over port 53 for any service that requires it.

The second object is a Replication Controller, which consists of several different containers that work together to provide DNS lookups. There's too much going on to explain it all, but it's using health checks, resource limits, and intra-pod networking over multiple ports.

Next, start the DNS add-on:

```sh
$ kubectl create -f dns-addon.yml
```

And check for `kube-dns-v17.1-*` pod up and running:

```sh
$ kubectl get pods --namespace=kube-system | grep kube-dns-v17.1
```

## Deploy the kube Dashboard Add-on

Create `kube-dashboard-rc.json` and `kube-dashboard-svc.json` on your local machine.

**kube-dashboard-rc.json**


```json
{
  "apiVersion": "v1",
  "kind": "ReplicationController",
  "metadata": {
    "labels": {
      "k8s-app": "kubernetes-dashboard",
      "kubernetes.io/cluster-service": "true",
      "version": "v1.1.1"
    },
    "name": "kubernetes-dashboard-v1.1.1",
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
          "version": "v1.1.1"
        }
      },
      "spec": {
        "containers": [
          {
            "image": "gcr.io/google_containers/kubernetes-dashboard-amd64:v1.1.1",
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
```

**kube-dashboard-svc.json**


```json
{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "labels": {
      "k8s-app": "kubernetes-dashboard",
      "kubernetes.io/cluster-service": "true"
    },
    "name": "kubernetes-dashboard",
    "namespace": "kube-system"
  },
  "spec": {
    "ports": [
      {
        "port": 80,
        "targetPort": 9090
      }
    ],
    "selector": {
      "k8s-app": "kubernetes-dashboard"
    }
  }
}
```

Create the Replication Controller and Service.

```sh
$ kubectl create -f kube-dashboard-rc.json
$ kubectl create -f kube-dashboard-svc.json
```

Access the dashboard by port forwarding with `kubectl`.


```sh
$ kubectl get pods
$ kubectl port-forward kubernetes-dashboard-v1.1.1-SOME-ID 9090 --namespace=kube-system
```

Then visit [http://127.0.0.1:9090](http://127.0.0.1:9090/) in your browser.

<div class="co-m-docs-next-step">
  <p>Now that you have a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.</p>
  <p>Start with a multi-tier web application (Guestbook) from the official Kubernetes documentation to visualize how the various Kubernetes components fit together.</p>
  <a href="https://github.com/kubernetes/kubernetes/blob/release-1.3/examples/guestbook/README.md" class="btn btn-default btn-icon-right" data-category="Docs Next" data-event="kubernetes.io: Guestbook">Deploy the Guestbook Sample app</a>
</div>
