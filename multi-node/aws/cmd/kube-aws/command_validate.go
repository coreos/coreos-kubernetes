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
		RunE:  runCmdValidate,
	}

	validateOpts = struct {
		awsDebug bool
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdValidate)
	cmdValidate.Flags().BoolVar(&validateOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Unable to load cluster config: %v", err)
	}

	//Validate cloudconfig userdata
	if err := cfg.ValidateUserData(stackTemplateOptions); err != nil {
		return err
	}

	fmt.Printf("UserData is valid\n")

	//Validate stack template
	data, err := cfg.RenderStackTemplate(stackTemplateOptions)
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	cluster := cluster.New(cfg, validateOpts.awsDebug)
	report, err := cluster.ValidateStack(string(data))
	if report != "" {
		fmt.Fprintf(os.Stderr, "Validation Report: %s\n", report)
	}

	if err != nil {
		return fmt.Errorf("Error creating cluster: %v", err)
	}
	fmt.Printf("Validation OK!\n")
	return nil
}
