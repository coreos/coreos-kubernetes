package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
)

var (
	cmdDestroy = &cobra.Command{
		Use:   "destroy",
		Short: "Destroy an existing Kubernetes cluster",
		Long:  ``,
		RunE:  runCmdDestroy,
	}
	destroyOpts = struct {
		awsDebug bool
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdDestroy)
	cmdDestroy.Flags().BoolVar(&destroyOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdDestroy(cmd *cobra.Command, args []string) error {
	cfg, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Error parsing config: %v", err)
	}

	c := cluster.New(cfg, destroyOpts.awsDebug)
	if err := c.Destroy(); err != nil {
		return fmt.Errorf("Failed destroying cluster: %v", err)
	}

	fmt.Println("Destroyed cluster")
	return nil
}
