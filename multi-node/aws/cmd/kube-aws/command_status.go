package main

import (
	"fmt"
	"os"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
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
	cfg, err := config.NewConfigFromFile(ConfigPath)
	if err != nil {
		stderr("Error parsing config: %v", err)
		os.Exit(1)
	}

	cluster := cluster.New(cfg, false)

	info, err := cluster.Info()
	if err != nil {
		stderr("Failed fetching cluster info: %v", err)
		os.Exit(1)
	}

	fmt.Print(info.String())
}
