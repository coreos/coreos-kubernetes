# Development

### Build from upstream release:

```
make all TAG=<custom-image-tag> KUBERNETES_VERSION=1.X.X
make push TAG=<custom-image-tag>
```

### Build using custom hyperkube

- Build hyperkube

    ```
    git clone https://github.com/kubernetes/kubernetes
    cd kubernetes
    # hack
    make all WHAT=cmd/hyperkube # NOTE: should be done on linux host (dynamic binary)
    ```

- build container using custom binary

    ```
    make container TAG=<image-tag> HYPERKUBE=<path-to-hyperkube-bin>
    make push TAG=<image-tag>
    ```

# Release

1. Update KUBERNETES_VERSION in Makefile
1. Build release candidate image

    ```
    make release TAG=quay.io/coreos/hyperkube:KUBERNETES_VERSION-rc.X
    make push TAG=quay.io/coreos/hyperkube:KUBERNETES_VERSION-rc.X
    ```

1. Test release candidate
1. Create pull-request & Merge
1. Build / Push release

    ```
    make release
    make push
    ```
