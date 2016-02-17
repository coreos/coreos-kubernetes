package config

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/coreos/coreos-cloudinit/config/validate"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/blobutil"
)

type UserDataConfig struct {
	Controller *blobutil.NamedBuffer
	Worker     *blobutil.NamedBuffer
	buffers    blobutil.NamedBufferList
}

func newUserDataConfig() *UserDataConfig {

	udc := &UserDataConfig{
		Controller: &blobutil.NamedBuffer{
			Name: "cloud-config-controller",
		},
		Worker: &blobutil.NamedBuffer{
			Name: "cloud-config-worker",
		},
	}

	udc.buffers = blobutil.NamedBufferList{
		udc.Controller,
		udc.Worker,
	}

	return udc
}

func (udc *UserDataConfig) generateDefaultConfigs() error {
	defaultConfigs := []struct {
		buffer          *blobutil.NamedBuffer
		defaultTemplate string
	}{
		{
			buffer:          udc.Controller,
			defaultTemplate: cloudConfigControllerTemplate,
		},
		{
			buffer:          udc.Worker,
			defaultTemplate: cloudConfigWorkerTemplate,
		},
	}

	for _, defaultConfig := range defaultConfigs {
		in := bytes.NewBuffer([]byte(defaultConfig.defaultTemplate))

		if _, err := defaultConfig.buffer.ReadFrom(in); err != nil {
			return fmt.Errorf("Error reading default config for %s : %v",
				defaultConfig.buffer.Name,
				err,
			)
		}
	}

	return nil
}

func (udc *UserDataConfig) validate() error {
	errors := []string{}

	for _, buffer := range udc.buffers {
		report, err := validate.Validate(buffer.Bytes())

		if err != nil {
			return fmt.Errorf("Cloud-config %s could not be parsed: %v",
				buffer.Name,
				err,
			)
		}

		for _, entry := range report.Entries() {
			errors = append(errors, fmt.Sprintf("%s: %+v", buffer.Name, entry))
		}

	}

	if len(errors) > 0 {
		reportString := strings.Join(errors, "\n")
		return fmt.Errorf("cloud-config validation errors:\n%s\n", reportString)
	}

	return nil
}
