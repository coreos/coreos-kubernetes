package cluster

import (
	"bytes"

	"fmt"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"io/ioutil"
	"path/filepath"
)

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

type TLSConfig struct {
	CACertFile string
	CACert     []byte

	APIServerCertFile string
	APIServerCert     []byte
	APIServerKeyFile  string
	APIServerKey      []byte

	WorkerCertFile string
	WorkerCert     []byte
	WorkerKeyFile  string
	WorkerKey      []byte

	AdminCertFile string
	AdminCert     []byte
	AdminKeyFile  string
	AdminKey      []byte
}

func New(cfg *Config, awsConfig *aws.Config) *Cluster {
	return &Cluster{
		cfg: cfg,
		aws: awsConfig,
	}
}

type Cluster struct {
	cfg *Config
	aws *aws.Config
}

func (c *Cluster) stackName() string {
	return c.cfg.ClusterName
}

func (c *Cluster) Create(tlsConfig *TLSConfig) error {
	parameters := []*cloudformation.Parameter{
		{
			ParameterKey:     aws.String(parClusterName),
			ParameterValue:   aws.String(c.stackName()),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parNameKeyName),
			ParameterValue:   aws.String(c.cfg.KeyName),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parInstallWorkerScript),
			ParameterValue:   aws.String(string(c.cfg.InstallWorkerScript)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parInstallControllerScript),
			ParameterValue:   aws.String(string(c.cfg.InstallControllerScript)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parClusterManifestsTar),
			ParameterValue:   aws.String(string(c.cfg.ClusterManifestsTar)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parControllerManifestsTar),
			ParameterValue:   aws.String(string(c.cfg.ControllerManifestsTar)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parWorkerManifestsTar),
			ParameterValue:   aws.String(string(c.cfg.WorkerManifestsTar)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parCACert),
			ParameterValue:   aws.String(string(tlsConfig.CACert)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parAPIServerCert),
			ParameterValue:   aws.String(string(tlsConfig.APIServerCert)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parAPIServerKey),
			ParameterValue:   aws.String(string(tlsConfig.APIServerKey)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parWorkerCert),
			ParameterValue:   aws.String(string(tlsConfig.WorkerCert)),
			UsePreviousValue: aws.Bool(true),
		},
		{
			ParameterKey:     aws.String(parWorkerKey),
			ParameterValue:   aws.String(string(tlsConfig.WorkerKey)),
			UsePreviousValue: aws.Bool(true),
		},
	}

	if c.cfg.ReleaseChannel != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parNameReleaseChannel),
			ParameterValue:   aws.String(c.cfg.ReleaseChannel),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.ControllerInstanceType != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parNameControllerInstanceType),
			ParameterValue:   aws.String(c.cfg.ControllerInstanceType),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.ControllerRootVolumeSize > 0 {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parNameControllerRootVolumeSize),
			ParameterValue:   aws.String(fmt.Sprintf("%d", c.cfg.ControllerRootVolumeSize)),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.WorkerCount > 0 {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parWorkerCount),
			ParameterValue:   aws.String(fmt.Sprintf("%d", c.cfg.WorkerCount)),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.WorkerInstanceType != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parNameWorkerInstanceType),
			ParameterValue:   aws.String(c.cfg.WorkerInstanceType),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.WorkerRootVolumeSize > 0 {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parNameWorkerRootVolumeSize),
			ParameterValue:   aws.String(fmt.Sprintf("%d", c.cfg.WorkerRootVolumeSize)),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.AvailabilityZone != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parAvailabilityZone),
			ParameterValue:   aws.String(c.cfg.AvailabilityZone),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.VPCCIDR != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parVPCCIDR),
			ParameterValue:   aws.String(c.cfg.VPCCIDR),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.InstanceCIDR != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parInstanceCIDR),
			ParameterValue:   aws.String(c.cfg.InstanceCIDR),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.ControllerIP != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parControllerIP),
			ParameterValue:   aws.String(c.cfg.ControllerIP),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.PodCIDR != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parPodCIDR),
			ParameterValue:   aws.String(c.cfg.PodCIDR),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.ServiceCIDR != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parServiceCIDR),
			ParameterValue:   aws.String(c.cfg.ServiceCIDR),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.KubernetesServiceIP != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parKubernetesServiceIP),
			ParameterValue:   aws.String(c.cfg.KubernetesServiceIP),
			UsePreviousValue: aws.Bool(true),
		})
	}

	if c.cfg.DNSServiceIP != "" {
		parameters = append(parameters, &cloudformation.Parameter{
			ParameterKey:     aws.String(parDNSServiceIP),
			ParameterValue:   aws.String(c.cfg.DNSServiceIP),
			UsePreviousValue: aws.Bool(true),
		})
	}

	tmplBody, err := ioutil.ReadFile(filepath.Join(c.cfg.ArtifactPath, "template.json"))
	if err != nil {
		return err
	}

	return createStackAndWait(cloudformation.New(c.aws), c.stackName(), string(tmplBody), parameters)
}

func (c *Cluster) Info() (*ClusterInfo, error) {
	resources, err := getStackResources(cloudformation.New(c.aws), c.stackName())
	if err != nil {
		return nil, err
	}

	info, err := mapStackResourcesToClusterInfo(ec2.New(c.aws), resources)
	if err != nil {
		return nil, err
	}

	info.Name = c.cfg.ClusterName
	return info, nil
}

func (c *Cluster) Destroy() error {
	return destroyStack(cloudformation.New(c.aws), c.stackName())
}
