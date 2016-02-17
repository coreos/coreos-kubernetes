package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
)

// set by build script
var VERSION = "UNKNOWN"

type ClusterInfo struct {
	Name         string
	ControllerIP string
}

func (c *ClusterInfo) String() string {
	buf := new(bytes.Buffer)
	w := new(tabwriter.Writer)
	w.Init(buf, 0, 8, 0, '\t', 0)

	fmt.Fprintf(w, "Cluster Name:\t%s\n", c.Name)
	fmt.Fprintf(w, "Controller IP:\t%s\n", c.ControllerIP)

	w.Flush()
	return buf.String()
}

func New(cfg *config.Config, awsDebug bool) *Cluster {

	//Set up AWS config
	awsConfig := aws.NewConfig()
	awsConfig = awsConfig.WithRegion(cfg.Region)
	if awsDebug {
		awsConfig = awsConfig.WithLogLevel(aws.LogDebug)
	}

	return &Cluster{
		cfg: cfg,
		aws: awsConfig,
	}
}

type Cluster struct {
	cfg *config.Config
	aws *aws.Config
}

func (c *Cluster) stackName() string {
	return c.cfg.ClusterName
}

func (c *Cluster) getStackBody() (string, error) {
	//Minify the JSON
	stackHolder := &map[string]interface{}{}
	if err := json.Unmarshal(c.cfg.StackTemplate.Bytes(), stackHolder); err != nil {
		return "", fmt.Errorf("Error unmarshalling stack json : %v", err)
	}

	miniStackBody, err := json.Marshal(stackHolder)
	if err != nil {
		return "", fmt.Errorf("Error marshalling stack json : %v", err)
	}

	return string(miniStackBody), nil
}

func (c *Cluster) ValidateStack() (string, error) {

	stackBody, err := c.getStackBody()
	if err != nil {
		return "", err
	}

	return validateStack(cloudformation.New(session.New(c.aws)), stackBody)
}

func (c *Cluster) Create() error {
	stackBody, err := c.getStackBody()
	if err != nil {
		return err
	}
	return createStackAndWait(cloudformation.New(session.New(c.aws)), c.stackName(), stackBody)
}

func (c *Cluster) Update() error {
	stackBody, err := c.getStackBody()
	if err != nil {
		return err
	}

	report, err := updateStack(cloudformation.New(session.New(c.aws)), c.stackName(), stackBody)

	fmt.Printf("Update stack: %s\n", report)
	return err
}

//TODO: validate cluster
func (c *Cluster) Info() (*ClusterInfo, error) {
	resources, err := getStackResources(cloudformation.New(session.New(c.aws)), c.stackName())
	if err != nil {
		return nil, err
	}

	info, err := mapStackResourcesToClusterInfo(ec2.New(session.New(c.aws)), resources)
	if err != nil {
		return nil, err
	}

	info.Name = c.cfg.ClusterName
	return info, nil
}

func (c *Cluster) Destroy() error {
	return destroyStack(cloudformation.New(session.New(c.aws)), c.stackName())
}
