# Kubernetes on CoreOS with Generic Install Scripts

This guide will setup Kubernetes on CoreOS in a similar way to other tools in the repo. The main goal of these scripts is to be generic and work on many different cloud providers or platforms. The notable difference is that these scripts are intended to be platform agnostic and thus don't automatically setup the TLS assets on each host beforehand.

[Read the documentation to boot a cluster][docs]

[docs]: /Documentation/kubernetes-on-generic-platforms.md