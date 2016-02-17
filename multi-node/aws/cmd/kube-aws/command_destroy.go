package main

import (
	"fmt"
	"os"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cmdDestroy = &cobra.Command{
		Use:   "destroy",
		Short: "Destroy an existing Kubernetes cluster",
		Long:  ``,
		Run:   runCmdDestroy,
	}
	destroyOpts = struct {
		awsDebug bool
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdDestroy)
	cmdDestroy.Flags().BoolVar(&destroyOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdDestroy(cmd *cobra.Command, args []string) {
	cfg, err := config.NewConfigFromFile(ConfigPath)
	if err != nil {
		stderr("Error parsing config: %v", err)
		os.Exit(1)
	}

	cluster := cluster.New(cfg, destroyOpts.awsDebug)

	if err := cluster.Destroy(); err != nil {
		stderr("Failed destroying cluster: %v", err)
		os.Exit(1)
	}

	fmt.Println("Destroyed cluster")
}
