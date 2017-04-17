export GO15VENDOREXPERIMENT=1

TAG = v0.0.0
PREFIX = quay.io/aaron_levy
GOOS = linux

.PHONY: bin container push clean

bin: bin/version-controller
release: clean container

bin/version-controller: cmd/version-controller.go
	mkdir -p bin
	GOPATH=$(CURDIR)/vendor/k8s.io/kubernetes/Godeps/_workspace:$(GOPATH) CGO_ENABLED=0 GOOS=$(GOOS) go build -o bin/version-controller cmd/version-controller.go

container: bin/version-controller
	docker $(DOCKER_FLAGS) build -t $(PREFIX)/version-controller:$(TAG) .

push: container
	docker push $(PREFIX)/version-controller:$(TAG)

clean:
	rm -rf bin

