package main

import (
	"fmt"
	"io/ioutil"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cmdUp = &cobra.Command{
		Use:   "up",
		Short: "Create a new Kubernetes cluster",
		Long:  ``,
		RunE:  runCmdUp,
	}

	upOpts = struct {
		awsDebug, export, update bool
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdUp)
	cmdUp.Flags().BoolVar(&upOpts.export, "export", false, "don't create cluster. instead export cloudformation stack file")
	//	cmdUp.Flags().BoolVar(&upOpts.update, "update", false, "update existing cluster with new cloudformation stack")
	cmdUp.Flags().BoolVar(&upOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdUp(cmd *cobra.Command, args []string) error {
	conf, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	data, err := conf.RenderStackTemplate(stackTemplateOptions)
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	if upOpts.export {
		templatePath := fmt.Sprintf("%s.stack-template.json", conf.ClusterName)
		fmt.Printf("Exporting %s\n", templatePath)
		if err := ioutil.WriteFile(templatePath, data, 0600); err != nil {
			return fmt.Errorf("Error writing %s : %v", templatePath, err)
		}
		fmt.Printf("BEWARE: %s contains your TLS secrets!\n", templatePath)
		return nil
	}
	cluster := cluster.New(conf, upOpts.awsDebug)
	if upOpts.update {
		report, err := cluster.Update(string(data))
		if err != nil {
			return fmt.Errorf("Error updating cluster: %v", err)
		}
		if report != "" {
			fmt.Printf("Update stack: %s\n", report)
		}
	} else {
		if err := cluster.Create(string(data)); err != nil {
			return fmt.Errorf("Error creating cluster: %v", err)
		}
	}

	info, err := cluster.Info()
	if err != nil {
		return fmt.Errorf("Failed fetching cluster info: %v", err)
	}

	fmt.Print(info.String())

	return nil
}
