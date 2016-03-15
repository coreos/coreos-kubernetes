package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cmdInit = &cobra.Command{
		Use:   "init",
		Short: "Initialize default kube-aws cluster configuration",
		Long:  ``,
		Run:   runCmdInit,
	}

	initOpts = config.Config{}
)

func init() {
	cmdRoot.AddCommand(cmdInit)
	cmdInit.Flags().StringVar(&initOpts.ClusterName, "cluster-name", "", "The name of this cluster. This will be the name of the cloudformation stack")
	cmdInit.Flags().StringVar(&initOpts.ExternalDNSName, "external-dns-name", "", "The hostname that will route to the api server")
	cmdInit.Flags().StringVar(&initOpts.Region, "region", "", "The aws region to deploy to")
	cmdInit.Flags().StringVar(&initOpts.AvailabilityZone, "availability-zone", "", "The aws availability-zone to deploy to")
	cmdInit.Flags().StringVar(&initOpts.KeyName, "key-name", "", "AWS key-pair for ssh access to nodes")
}

func runCmdInit(cmd *cobra.Command, args []string) {
	if initOpts.ClusterName == "" {
		stderr("Must provide cluster-name parameter")
		os.Exit(1)
	}
	if initOpts.ExternalDNSName == "" {
		stderr("Must provide external-dns-name parameter")
		os.Exit(1)
	}
	if initOpts.Region == "" {
		stderr("Must provide region parameter")
		os.Exit(1)
	}
	if initOpts.AvailabilityZone == "" {
		stderr("Must provide availability zone parameter")
		os.Exit(1)
	}
	if initOpts.KeyName == "" {
		stderr("Must provide key-name parameter")
		os.Exit(1)
	}

	cfgTemplate, err := template.New("cluster.yaml").Parse(config.DefaultClusterConfig)
	if err != nil {
		stderr("Error parsing default config template: %v", err)
		os.Exit(1)
	}

	out, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		stderr("Error opening %s : %v", configPath, err)
		os.Exit(1)
	}
	defer out.Close()
	if err := cfgTemplate.Execute(out, initOpts); err != nil {
		stderr("Error exec-ing default config template: %v", err)
		os.Exit(1)
	}

	fmt.Printf("Edit %s to parameterize the cluster. Then use the \"kube-aws render\" command to render the stack template\n", configPath)
}
