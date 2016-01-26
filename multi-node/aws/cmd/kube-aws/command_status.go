package main

import (
	"fmt"
	"os"

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
	c, err := cluster.New(rootOpts.AssetDir, rootOpts.AWSDebug)
	if err != nil {
		stderr("Invalid cluster assets: %v", err)
		os.Exit(1)
	}

	info, err := c.Info()
	if err != nil {
		stderr("Failed fetching cluster info: %v", err)
		os.Exit(1)
	}

	fmt.Print(info.String())
}
