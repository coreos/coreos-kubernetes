package main

import (
	"fmt"
	"os"
	"path/filepath"

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
	cfgPath := filepath.Join(rootOpts.AssetDir, "cluster.yaml")
	cfg := cluster.NewDefaultConfig(VERSION)
	err := cluster.DecodeConfigFromFile(cfg, cfgPath)
	if err != nil {
		stderr("Unable to load cluster config: %v", err)
		os.Exit(1)
	}

	c := cluster.New(cfg, newAWSConfig(cfg))

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
