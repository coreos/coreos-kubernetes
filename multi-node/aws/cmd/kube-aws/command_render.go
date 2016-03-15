package main

import (
	"fmt"
	"os"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cmdRender = &cobra.Command{
		Use:   "render",
		Short: "Render a CloudFormation template",
		Long:  ``,
		Run:   runCmdRender,
	}
)

func init() {
	cmdRoot.AddCommand(cmdRender)
}

func runCmdRender(cmd *cobra.Command, args []string) {
	cfg, err := config.NewConfigFromFile(configPath)
	if err != nil {
		stderr("Error parsing config from file: %v", err)
		os.Exit(1)
	}

	if err := cfg.GenerateDefaultAssets(); err != nil {
		stderr("Error generating default assets : %v", err)
		os.Exit(1)
	}

	if err := cfg.KubeConfig.Template(cfg); err != nil {
		stderr("Error templating kubeconfig : %v", err)
		os.Exit(1)
	}

	if err := cfg.WriteAssetsToFiles(); err != nil {
		stderr("Error writing assets to file: %v", err)
		os.Exit(1)
	}

	fmt.Printf("Edit %s and/or any of the cluster assets. Then use the \"kube-aws up\" command to create the stack\n", configPath)
}
