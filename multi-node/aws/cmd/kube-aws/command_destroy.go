package main

import (
	"fmt"
	"os"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/spf13/cobra"
)

var (
	cmdDestroy = &cobra.Command{
		Use:   "destroy",
		Short: "Destroy an existing Kubernetes cluster",
		Long:  ``,
		Run:   runCmdDestroy,
	}
)

func init() {
	cmdRoot.AddCommand(cmdDestroy)
}

func runCmdDestroy(cmd *cobra.Command, args []string) {
	c, err := cluster.New(rootOpts.AssetDir, rootOpts.AWSDebug)
	if err != nil {
		stderr("Invalid cluster assets: %v", err)
		os.Exit(1)
	}

	if err := c.Destroy(); err != nil {
		stderr("Failed destroying cluster: %v", err)
		os.Exit(1)
	}

	if err := os.RemoveAll(rootOpts.AssetDir); err != nil {
		stderr("Failed removing local cluster directory: %v", err)
		os.Exit(1)
	}

	fmt.Println("Destroyed cluster")
}
