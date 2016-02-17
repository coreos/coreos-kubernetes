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
)

const ConfigPath = "./cluster.yaml"

func main() {
	cmdRoot.Execute()
}

func stderr(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
