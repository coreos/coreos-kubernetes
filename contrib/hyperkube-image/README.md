# Development

### Build from upstream release:

```
make container TAG=<custom-image-tag> KUBERNETES_VERSION=<release-version>
make push TAG=<custom-image-tag>
```

### Build using custom hyperkube

- Build hyperkube

    ```
    git clone https://github.com/kubernetes/kubernetes
    cd kubernetes
    # hack
    KUBE_BUILD_PLATFORMS=linux/amd64 make all WHAT=cmd/hyperkube
    ```

- build container using custom binary

    ```
    make container TAG=<image-tag> BIN=<path-to-hyperkube-bin>
    make push TAG=<image-tag>
    ```

# Release

1. Update KUBELET_VERSION in Makefile
1. Create pull-request & Merge
1. Build / Push release

    ```
    make clean
    make container
    make push
    ```
