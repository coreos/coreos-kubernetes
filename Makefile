CONTROLLER_CLOUD_CONFIGS = \
	multi-node/generic/controller-cloud-config.yaml \
	multi-node/vagrant/controller-cloud-config.yaml \
	single-node/cloud-config.yaml

WORKER_CLOUD_CONFIGS = \
	multi-node/generic/worker-cloud-config.yaml \
	multi-node/vagrant/worker-cloud-config.yaml

all: $(CONTROLLER_CLOUD_CONFIGS) $(WORKER_CLOUD_CONFIGS)

$(CONTROLLER_CLOUD_CONFIGS): deploy/controller.sh
	@echo "CONFIG: $@"
	@sed -e 's/^/    /' -e 's/^ *$$//' $< | cat $@.tmpl - > $@

$(WORKER_CLOUD_CONFIGS): deploy/worker.sh
	@echo "CONFIG: $@"
	@sed -e 's/^/    /' -e 's/^ *$$//' $< | cat $@.tmpl - > $@
