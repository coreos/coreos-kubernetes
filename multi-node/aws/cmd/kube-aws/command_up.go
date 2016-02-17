package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cmdUp = &cobra.Command{
		Use:   "up",
		Short: "Create a new Kubernetes cluster",
		Long:  ``,
		Run:   runCmdUp,
	}

	upOpts = struct {
		awsDebug, export, update bool
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdUp)
	cmdUp.Flags().BoolVar(&upOpts.export, "export", false, "don't create cluster. instead export cloudformation stack file")
	cmdUp.Flags().BoolVar(&upOpts.update, "update", false, "update existing cluster with new cloudformation stack")
	cmdUp.Flags().BoolVar(&upOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdUp(cmd *cobra.Command, args []string) {
	cfg, err := config.NewConfigFromFile(ConfigPath)
	if err != nil {
		stderr("Unable to load cluster config: %v", err)
		os.Exit(1)
	}

	if err := cfg.ReadAssetsFromFiles(); err != nil {
		stderr("Error reading assets from files: %v", err)
		os.Exit(1)
	}

	if err := cfg.TemplateAndEncodeAssets(); err != nil {
		stderr("Error templating assets: %v", err)
		os.Exit(1)
	}
	if upOpts.export {
		templatePath := fmt.Sprintf("./%s.stack-template.json", cfg.ClusterName)
		fmt.Printf("Exporting %s\n", templatePath)
		if err := ioutil.WriteFile(templatePath, cfg.StackTemplate.Bytes(), 0600); err != nil {
			stderr("Error writing %s : %v", templatePath, err)
			os.Exit(1)
		}
		fmt.Printf("BEWARE: %s contains your TLS secrets!\n", templatePath)
		os.Exit(0)
	}
	cluster := cluster.New(cfg, upOpts.awsDebug)

	if upOpts.update {
		if err := cluster.Update(); err != nil {
			stderr("Error updating cluster: %v", err)
			os.Exit(1)
		}
	} else {
		if err := cluster.Create(); err != nil {
			stderr("Error creating cluster: %v", err)
			os.Exit(1)
		}
	}

	info, err := cluster.Info()
	if err != nil {
		stderr("Failed fetching cluster info: %v", err)
		os.Exit(1)
	}

	fmt.Print(info.String())

}
