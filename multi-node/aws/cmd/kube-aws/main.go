package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/spf13/cobra"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
)

var (
	// set by build script
	VERSION = "UNKNOWN"
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

func newAWSConfig(cfg *cluster.Config) *aws.Config {
	c := aws.NewConfig()
	c = c.WithRegion(cfg.Region)
	if rootOpts.AWSDebug {
		c = c.WithLogLevel(aws.LogDebug)
	}
	return c
}
