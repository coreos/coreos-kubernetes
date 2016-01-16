package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/spf13/cobra"
)

var (
	cmdStatus = &cobra.Command{
		Use:   "status",
		Short: "Describe an existing Kubernetes cluster",
		Long:  ``,
		Run:   runCmdStatus,
	}
)

func init() {
	cmdRoot.AddCommand(cmdStatus)
}

func runCmdStatus(cmd *cobra.Command, args []string) {
	cfgPath := filepath.Join(rootOpts.AssetDir, "cluster.yaml")
	cfg := cluster.NewDefaultConfig(VERSION)
	err := cluster.DecodeConfigFromFile(cfg, cfgPath)
	if err != nil {
		stderr("Unable to load cluster config: %v", err)
		os.Exit(1)
	}

	c := cluster.New(cfg, newAWSConfig(cfg))

	info, err := c.Info()
	if err != nil {
		stderr("Failed fetching cluster info: %v", err)
		os.Exit(1)
	}

	fmt.Print(info.String())
}
