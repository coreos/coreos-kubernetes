package main

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
)

var (
	cmdDestroy = &cobra.Command{
		Use:   "destroy",
		Short: "Destroy an existing Kubernetes cluster",
		Long:  ``,
		Run:   runCmdDestroy,
	}

	force bool
)

func init() {
	cmdRoot.AddCommand(cmdDestroy)
	cmdDestroy.Flags().BoolVar(&force, "force", false, "Destroy the cluster without interactive confirmation")
}

func runCmdDestroy(cmd *cobra.Command, args []string) {
	cfg := cluster.NewDefaultConfig(VERSION)
	err := cluster.DecodeConfigFromFile(cfg, rootOpts.ConfigPath)
	if err != nil {
		stderr("Unable to load cluster config: %v", err)
		os.Exit(1)
	}

	c := cluster.New(cfg, newAWSConfig(cfg))

	if !force {
		var confirmation string

		fmt.Print("Are you sure you want to destroy the cluster? This action cannot be undone. Type \"yes\" to proceed: ")
		if _, err = fmt.Scanln(&confirmation); err != nil {
			stderr("Failed to read user input: %v", err)
			os.Exit(1)
		}

		if confirmation != "yes" {
			os.Exit(1)
		}
	}

	if err := c.Destroy(); err != nil {
		stderr("Failed destroying cluster: %v", err)
		os.Exit(1)
	}

	clusterDir := path.Join("clusters", cfg.ClusterName)
	if err := os.RemoveAll(clusterDir); err != nil {
		stderr("Failed removing local cluster directory: %v", err)
		os.Exit(1)
	}

	fmt.Println("Destroyed cluster")
}
