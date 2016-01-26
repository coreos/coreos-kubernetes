package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cmdRoot = &cobra.Command{
		Use:   "kube-aws",
		Short: "Manage Kubernetes clusters on AWS",
		Long:  ``,
	}

	rootOpts struct {
		AWSDebug bool
		AssetDir string
	}
)

func init() {
	cmdRoot.PersistentFlags().BoolVar(&rootOpts.AWSDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdRoot.PersistentFlags().StringVar(&rootOpts.AssetDir, "asset-dir", "", "Folder (to be) created by 'render' command for this cluster's assets.")
}

func main() {
	cmdRoot.Execute()
}

func stderr(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
