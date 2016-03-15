package main

import (
	"fmt"
	"os"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cmdValidate = &cobra.Command{
		Use:   "validate",
		Short: "Validate cluster assets",
		Long:  ``,
		Run:   runCmdValidate,
	}

	validateOpts = struct {
		awsDebug bool
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdValidate)
	cmdValidate.Flags().BoolVar(&validateOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdValidate(cmd *cobra.Command, args []string) {
	cfg, err := config.NewConfigFromFile(configPath)
	if err != nil {
		stderr("Unable to load cluster config: %v", err)
		os.Exit(1)
	}

	if err := cfg.ReadAssetsFromFiles(); err != nil {
		stderr("Error reading assets from files: %v", err)
		os.Exit(1)
	}

	if err := cfg.TemplateAndEncodeAssets(); err != nil {
		stderr("template/encode error: %v", err)
		os.Exit(1)
	}

	cluster := cluster.New(cfg, validateOpts.awsDebug)

	report, err := cluster.ValidateStack()

	if report != "" {
		fmt.Printf("Validation Report: %s\n", report)
	}

	if err != nil {
		stderr("Error creating cluster: %v", err)
		os.Exit(1)
	}
	fmt.Printf("Validation OK!\n")
}
