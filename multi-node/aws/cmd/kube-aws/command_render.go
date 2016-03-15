package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cmdRender = &cobra.Command{
		Use:   "render",
		Short: "Render a CloudFormation template",
		Long:  ``,
		RunE:  runCmdRender,
	}
)

func init() {
	cmdRoot.AddCommand(cmdRender)
}

func runCmdRender(cmd *cobra.Command, args []string) error {
	// Read the config from file.
	cluster, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	// Generate default TLS assets.
	assets, err := cluster.NewTLSAssets()
	if err != nil {
		return fmt.Errorf("Error generating default assets: %v", err)
	}
	if err := os.Mkdir("credentials", 0700); err != nil {
		return err
	}
	if err := assets.WriteToDir("./credentials"); err != nil {
		return fmt.Errorf("Error create assets: %v", err)
	}

	// Create a Config and attempt to render a kubeconfig for it.
	cfg, err := cluster.Config(assets)
	if err != nil {
		return fmt.Errorf("Failed to create config: %v", err)
	}
	tmpl, err := template.New("kubeconfig.yaml").Parse(string(config.KubeConfigTemplate))
	if err != nil {
		return fmt.Errorf("Failed to parse default kubeconfig template: %v", err)
	}
	var kubeconfig bytes.Buffer
	if err := tmpl.Execute(&kubeconfig, cfg); err != nil {
		return fmt.Errorf("Failed to render kubeconfig: %v", err)
	}

	// Write all assets to disk.
	userdataDir := "userdata"
	if err := os.Mkdir(userdataDir, 0755); err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{"credentials/.gitignore", []byte("*"), 0644},
		{"userdata/cloud-config-controller", config.CloudConfigController, 0644},
		{"userdata/cloud-config-worker", config.CloudConfigWorker, 0644},
		{"stack-template.json", config.StackTemplateTemplate, 0644},
		{"kubeconfig", kubeconfig.Bytes(), 0600},
	}
	for _, file := range files {
		if err := ioutil.WriteFile(file.name, file.data, file.mode); err != nil {
			return err
		}
	}

	fmt.Printf("Edit %s and/or any of the cluster assets. Then use the \"kube-aws up\" command to create the stack\n", configPath)
	return nil
}
