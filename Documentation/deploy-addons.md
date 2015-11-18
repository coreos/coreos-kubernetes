## Deploy the DNS Add-on

The DNS add-on allows your services to have a DNS name in addition to an IP address. This is helpful for simplified service discovery between applications. More info can be found in the [Kubernetes DNS documentation][k8s-dns].

Add-ons are built on the same Kubernetes components as user-submitted jobs &mdash; Pods, Replication Controllers and Services. We're going to install the DNS add-on with `kubectl`.

First create `dns-addon.yml` on your local machine and replace the variable. There is a lot going on in there, so let's break it down after you create it.

[k8s-dns]: http://kubernetes.io/v1.1/docs/admin/dns.html

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
  name: kube-dns-v9
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    version: v9
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 1
  selector:
    k8s-app: kube-dns
    version: v9
  template:
    metadata:
      labels:
        k8s-app: kube-dns
        version: v9
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
      - name: etcd
        image: gcr.io/google_containers/etcd:2.0.9
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        command:
        - /usr/local/bin/etcd
        - -data-dir
        - /var/etcd/data
        - -listen-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -advertise-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -initial-cluster-token
        - skydns-etcd
        volumeMounts:
        - name: etcd-storage
          mountPath: /var/etcd/data
      - name: kube2sky
        image: gcr.io/google_containers/kube2sky:1.11
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        args:
        # command = "/kube2sky"
        - -domain=cluster.local
      - name: skydns
        image: gcr.io/google_containers/skydns:2015-03-11-001
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
        args:
        # command = "/skydns"
        - -machines=http://localhost:4001
        - -addr=0.0.0.0:53
        - -domain=cluster.local.
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 1
          timeoutSeconds: 5
      - name: healthz
        image: gcr.io/google_containers/exechealthz:1.0
        resources:
          limits:
            cpu: 10m
            memory: 20Mi
        args:
        - -cmd=nslookup kubernetes.default.svc.cluster.local localhost >/dev/null
        - -port=8080
        ports:
        - containerPort: 8080
          protocol: TCP
      volumes:
      - name: etcd-storage
        emptyDir: {}
      dnsPolicy: Default
```

This single YAML file is actually creating 2 different Kubernetes objects, separated by `---`.

The first object is a service that provides DNS lookups over port 53 for any service that requires it.

The second object is a Replication Controller, which consists of several different containers that work together to provide DNS lookups. There's too much going on to explain it all, but it's using health checks, resource limits, and intra-pod networking over multiple ports.

Next, start the DNS add-on:

```sh
$ kubectl create -f dns-addon.yml
```

And check for `kube-dns-v9-*` pod up and running:

```sh
$ kubectl get pods --namespace=kube-system | grep kube-dns-v9
```

<div class="co-m-docs-next-step">
  <p>Now that you have a working Kubernetes cluster with a functional CLI tool, you are free to deploy Kubernetes-ready applications.</p>
  <p>Start with a multi-tier web application (Guestbook) from the official Kubernetes documentation to visualize how the various Kubernetes components fit together.</p>
  <a href="http://kubernetes.io/v1.1/examples/guestbook-go/README.html" class="btn btn-default btn-icon-right" data-category="Docs Next" data-event="kubernetes.io: Guestbook">Deploy the Guestbook Sample app</a>
</div>
