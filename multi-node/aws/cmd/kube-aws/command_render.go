package main

import (
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
)

var (
	cmdRender = &cobra.Command{
		Use:   "render",
		Short: "Render a CloudFormation template",
		Long:  ``,
		Run:   runCmdRender,
	}

	renderOpts struct {
		Output string
	}
)

func init() {
	cmdRoot.AddCommand(cmdRender)
	cmdRender.Flags().StringVar(&renderOpts.Output, "output", "", "Write output to file instead of stdout")
}

func runCmdRender(cmd *cobra.Command, args []string) {
	tmpl, err := cluster.StackTemplateBody()
	if err != nil {
		stderr("Failed to generate template: %v", err)
		os.Exit(1)
	}

	if renderOpts.Output == "" {
		os.Stdout.WriteString(tmpl)
	} else {
		if err := ioutil.WriteFile(renderOpts.Output, []byte(tmpl), 0600); err != nil {
			stderr("Failed writing output to %s: %v", renderOpts.Output, err)
			os.Exit(1)
		}
	}
}
