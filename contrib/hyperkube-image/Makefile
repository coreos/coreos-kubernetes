KUBERNETES_VERSION = 1.1.2
KUBERNETES_RELEASE = https://github.com/kubernetes/kubernetes/archive/v$(KUBERNETES_VERSION).tar.gz

TAG = quay.io/coreos/hyperkube:v$(KUBERNETES_VERSION)
WORKDIR = _workdir
HYPERKUBE = $(WORKDIR)/$(KUBERNETES_VERSION)/_output/dockerized/bin/linux/amd64/hyperkube

all: $(HYPERKUBE) container
release: clean $(HYPERKUBE) container

# Patches to apply when building hyperkube from kubernetes release
#    Example: files/0001-foo-bar.patch
#    Patches can be generated from `git-format-patch`
PATCHES = \
	files/0001-kubelet-report-NodeReady-last-in-status-list.patch \
	files/0002-explicitly-check-Ready-condition-in-validate-cluster.patch \
	files/0003-kubelet-check-node-condition-by-type-rather-than-by-.patch \
	files/0004-pkg-kubelet-force-NodeReady-condition-to-be-last-on-.patch

# Build binaries with PATCHES using the upstream release process
# TODO(aaron): Patch upstream to more easily allow single-binary builds in the container based build process (make release).
$(HYPERKUBE): $(WORKDIR)/$(KUBERNETES_VERSION)
	@$(foreach p,$(PATCHES),patch --directory $(WORKDIR)/$(KUBERNETES_VERSION) --strip=1 < $(CURDIR)/$(p);)
	#TODO(aaron): change 'quick-release' to 'release'. Known unit tests failures (v1.1.2) preventing this.
	$(MAKE) -C $(WORKDIR)/$(KUBERNETES_VERSION) quick-release

# Build Docker container using HYPERKUBE binary
# If building with a custom binary:
#	make container HYPERKUBE=path-to-bin
container:
	$(eval TEMPDIR := $(shell mktemp -d -t $(KUBERNETES_VERSION)))
	echo $(TEMPDIR)
	cp ./Dockerfile $(TEMPDIR)
	cp $(HYPERKUBE) $(TEMPDIR)
	docker build -t $(TAG) $(TEMPDIR)
	rm -rf $(TEMPDIR)

push:
	docker push $(TAG)

# Download kubernetes release tarball
$(WORKDIR)/$(KUBERNETES_VERSION).tar.gz:
	mkdir -p $(WORKDIR)
	curl --silent --location https://github.com/kubernetes/kubernetes/archive/v$(KUBERNETES_VERSION).tar.gz > $@

# Extract kubernetes release to workdir/version
$(WORKDIR)/$(KUBERNETES_VERSION): $(WORKDIR)/$(KUBERNETES_VERSION).tar.gz
	rm -rf $@
	tar xf $(WORKDIR)/$(KUBERNETES_VERSION).tar.gz -C $(WORKDIR)
	mv $(WORKDIR)/kubernetes-$(KUBERNETES_VERSION) $@

clean:
	rm -rf $(WORKDIR)

.PHONY: all release container push clean
