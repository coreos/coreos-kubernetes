package main

import (
	"fmt"
	"os"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/spf13/cobra"
)

var (
	cmdUp = &cobra.Command{
		Use:   "up",
		Short: "Create a new Kubernetes cluster",
		Long:  ``,
		Run:   runCmdUp,
	}
)

func init() {
	cmdRoot.AddCommand(cmdUp)
}

func runCmdUp(cmd *cobra.Command, args []string) {
	c, err := cluster.New(rootOpts.AssetDir, rootOpts.AWSDebug)
	if err != nil {
		stderr("Invalid cluster assets: %v", err)
		os.Exit(1)
	}
	if err := c.Create(); err != nil {
		stderr("Failed creating cluster: %v", err)
		os.Exit(1)
	}

	fmt.Println("Successfully created cluster")
	fmt.Println("")

	info, err := c.Info()
	if err != nil {
		stderr("Failed fetching cluster info: %v", err)
		os.Exit(1)
	}

	fmt.Printf(info.String())
}
