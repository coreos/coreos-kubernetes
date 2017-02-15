# Kubernetes kubectl Tips and Tricks

This is a collection of tips and tricks for the [kubectl](https://coreos.com/kubernetes/docs/latest/configure-kubectl.html) command line tool. All of these tricks assume you have a working Kubernetes cluster and kubectl tool.

## Pod Operations with kubectl

### Filter Kubernetes Pods by Time

```
kubectl get pods --all-namespaces --sort-by='.metadata.creationTimestamp' -o jsonpath='{range .items[*]}{.metadata.name}, {.metadata.creationTimestamp}{"\n"}{end}'
```

### Find Kubernetes Pod by Label Selector and Fetch the Pod Logs

Given a namespace "your-namespace" and a label query that identifies the pods you are interested in you can get the logs for all of those pods. If the pod isn't unique it will fetch the logs for each pod in turn.

```
kubectl get pods --namespace=your-namespace -l run=hello-world -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | xargs -I {} kubectl -n your-namespace logs {}
```

### Find a Kubernetes Pod by Label Selector and Port-forward Locally

Given a namespace "your-namespace" and a label query that identifies the pods you are interested in connect to a particular pod instance. If the label selector doesn't find a unique pod it will connect to the first pod by name. Ensure you replace 8080 with your pod's port.

```
kubectl -n your-namespace get pods -n your-namespace -l run=hello-world -o jsonpath='{.items[1].metadata.name}' | xargs -I{} kubectl -n your-namespace port-forward {} 8080
```

## Node Operations with kubectl

The `jq` tool is a lightweight JSON processor that can do comparisons. By combining `jq` with `kubectl` JSON output you can make complex queries like filtering all resources by their create date.

### Count the Number of Pods on a Kubernetes Node

Often high level statistics can help in debugging. This command line will count up all of the pods on each node.

```
kubectl get pods --all-namespaces -o json | jq '.items[] | .spec.nodeName' -r | sort | uniq -c
```

### Get a List of Pods for Each Node

This will generate a JSON document that has a Kubernetes Node name and then a list of all of the pod names running on the node. Very useful for debugging placement or load issues.

```
kubectl get pods --all-namespaces -o json | jq '.items | map({podName: .metadata.name, nodeName: .spec.nodeName}) | group_by(.nodeName) | map({nodeName: .[0].nodeName, pods: map(.podName)})'
```

### Get the External IP for Kubernetes Nodes

```
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name} {.status.addresses[?(@.type=="ExternalIP")].address}{"\n"}{end}'
```

### SSH into Nodes with Fabric

Kubernetes has a database of nodes in the cluster which can be queried with `kub
ectl get nodes`. This is a powerful database for automation and integration with
 existing tools. One powerful tool is the [Fabric SSH utility](http://www.fabfil
e.org/) which is known as a `fabfile.py`.

There is a simple project introduced by CoreOS which integrates Kubernetes Nodes and fabric together enabling really powerful tools like ssh'ing into all machines running in a particular AWS failure domain.

```
fab -u core -R failure-domain.beta.kubernetes.io/zone=us-west-2a -- date
```

Learn more at the [Fabric Kubernetes Nodes project](https://github.com/coreos/fabric-kubernetes-nodes).
