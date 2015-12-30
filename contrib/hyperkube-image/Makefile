KUBERNETES_VERSION = 1.1.2
KUBERNETES_RELEASE = https://github.com/kubernetes/kubernetes/archive/v$(KUBERNETES_VERSION).tar.gz

TAG = quay.io/coreos/hyperkube:v$(KUBERNETES_VERSION)
WORKDIR = _workdir
BIN = $(WORKDIR)/$(KUBERNETES_VERSION)/_output/local/bin/linux/amd64/hyperkube

export KUBE_BUILD_PLATFORMS=linux/amd64

all: container
hyperkube: $(BIN)

# Patches to apply when building hyperkube from kubernetes release
PATCHES = \
	files/0001-kubelet-report-NodeReady-last-in-status-list.patch \
	files/0002-explicitly-check-Ready-condition-in-validate-cluster.patch \
	files/0003-kubelet-check-node-condition-by-type-rather-than-by-.patch \
	files/0004-pkg-kubelet-force-NodeReady-condition-to-be-last-on-.patch

# Build hyperkube from release version & apply $PATCHES
$(WORKDIR)/$(KUBERNETES_VERSION)/_output/local/bin/linux/amd64/hyperkube: $(WORKDIR)/$(KUBERNETES_VERSION) $(PATCHES)
	@$(foreach p,$(PATCHES),patch --directory $(WORKDIR)/$(KUBERNETES_VERSION) --strip=1 < $(CURDIR)/$(p);)
	$(MAKE) -C $(WORKDIR)/$(KUBERNETES_VERSION) all WHAT=cmd/hyperkube

# Build Docker container using hyperkube from $BIN
container: hyperkube
	$(eval TEMPDIR := $(shell mktemp -d -t $(KUBERNETES_VERSION)))
	echo $(TEMPDIR)
	cp ./Dockerfile $(TEMPDIR)
	cp $(BIN) $(TEMPDIR)
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

.PHONY: all hyperkube container push clean
