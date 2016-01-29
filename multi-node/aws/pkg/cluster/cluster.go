package cluster

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/blobutil"
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

type TLSConfig struct {
	CACertFile string
	CACert     []byte
	CAKeyFile  string
	CAKey      []byte

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

func NewTLSConfig(clusterDir string) *TLSConfig {
	return &TLSConfig{
		CACertFile:        path.Join(clusterDir, "ca.pem"),
		CAKeyFile:         path.Join(clusterDir, "ca-key.pem"),
		APIServerCertFile: path.Join(clusterDir, "apiserver.pem"),
		APIServerKeyFile:  path.Join(clusterDir, "apiserver-key.pem"),
		WorkerCertFile:    path.Join(clusterDir, "worker.pem"),
		WorkerKeyFile:     path.Join(clusterDir, "worker-key.pem"),
		AdminCertFile:     path.Join(clusterDir, "admin.pem"),
		AdminKeyFile:      path.Join(clusterDir, "admin-key.pem"),
	}
}

func (tc *TLSConfig) ReadFilesFromPaths() {
	tc.CACert = blobutil.MustReadAndCompressFile(tc.CACertFile)
	tc.CAKey = blobutil.MustReadAndCompressFile(tc.CAKeyFile)
	tc.APIServerCert = blobutil.MustReadAndCompressFile(tc.APIServerCertFile)
	tc.APIServerKey = blobutil.MustReadAndCompressFile(tc.APIServerKeyFile)
	tc.WorkerCert = blobutil.MustReadAndCompressFile(tc.WorkerCertFile)
	tc.WorkerKey = blobutil.MustReadAndCompressFile(tc.WorkerKeyFile)
	tc.AdminCert = blobutil.MustReadAndCompressFile(tc.AdminCertFile)
	tc.AdminKey = blobutil.MustReadAndCompressFile(tc.AdminKeyFile)
}

func New(assetDir string, awsDebug bool) (*Cluster, error) {
	cfgPath := filepath.Join(assetDir, "cluster.yaml")
	cfg := NewDefaultConfig()
	if err := DecodeConfigFromFile(cfg, cfgPath); err != nil {
		return nil, fmt.Errorf("Unable to load cluster config: %v", err)
	}

	awsConfig := aws.NewConfig()
	awsConfig = awsConfig.WithRegion(cfg.Region)
	if awsDebug {
		awsConfig = awsConfig.WithLogLevel(aws.LogDebug)
	}

	c := &Cluster{
		cfg:      cfg,
		aws:      awsConfig,
		assetDir: assetDir,
	}
	return c, nil
}

type Cluster struct {
	cfg      *Config
	aws      *aws.Config
	assetDir string
}

func (c *Cluster) stackName() string {
	return c.cfg.ClusterName
}

func (c *Cluster) initAssets() *TLSConfig {

	c.cfg.InstallWorkerScript = blobutil.MustReadAndCompressFile(filepath.Join(c.assetDir, "scripts", "install-worker.sh"))
	c.cfg.InstallControllerScript = blobutil.MustReadAndCompressFile(filepath.Join(c.assetDir, "scripts", "install-controller.sh"))

	manifestPath := filepath.Join(c.assetDir, "manifests")
	c.cfg.ClusterManifestsTar = blobutil.MustTarAndCompressDirectory(c.assetDir, filepath.Join(manifestPath, "cluster"))
	c.cfg.ControllerManifestsTar = blobutil.MustTarAndCompressDirectory(c.assetDir, filepath.Join(manifestPath, "controller"))
	c.cfg.WorkerManifestsTar = blobutil.MustTarAndCompressDirectory(c.assetDir, filepath.Join(manifestPath, "worker"))

	credentialsDir := filepath.Join(c.assetDir, "credentials")
	tlsConfig := NewTLSConfig(credentialsDir)
	tlsConfig.ReadFilesFromPaths()
	return tlsConfig
}

func (c *Cluster) Create() error {

	tlsConfig := c.initAssets()
	fmt.Printf("Cluster assets initialized from '%s'\n", c.assetDir)
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

	tmplBody, err := ioutil.ReadFile(filepath.Join(c.assetDir, "template.json"))
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
