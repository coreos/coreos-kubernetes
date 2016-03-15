package config

//go:generate go run templates_gen.go
//go:generate gofmt -w templates.go

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"text/template"

	"github.com/coreos/coreos-cloudinit/config/validate"
	"gopkg.in/yaml.v2"
)

const (
	credentialsDir = "credentials"
	userDataDir    = "userdata"
)

func newDefaultCluster() *Cluster {
	return &Cluster{
		ClusterName:              "kubernetes",
		ReleaseChannel:           "alpha",
		VPCCIDR:                  "10.0.0.0/16",
		InstanceCIDR:             "10.0.0.0/24",
		ControllerIP:             "10.0.0.50",
		PodCIDR:                  "10.2.0.0/16",
		ServiceCIDR:              "10.3.0.0/24",
		KubernetesServiceIP:      "10.3.0.1",
		DNSServiceIP:             "10.3.0.10",
		K8sVer:                   "v1.1.4",
		ControllerInstanceType:   "m3.medium",
		ControllerRootVolumeSize: 30,
		WorkerCount:              1,
		WorkerInstanceType:       "m3.medium",
		WorkerRootVolumeSize:     30,
	}
}

func ClusterFromFile(filename string) (*Cluster, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c, err := clusterFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("file %s: %v", filename, err)
	}

	return c, nil
}

//Necessary for unit tests, which store configs as hardcoded strings
func clusterFromBytes(data []byte) (*Cluster, error) {
	c := newDefaultCluster()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("failed to parse cluster: %v", err)
	}
	if err := c.valid(); err != nil {
		return nil, fmt.Errorf("invalid cluster: %v", err)
	}
	return c, nil
}

type Cluster struct {
	ClusterName              string `yaml:"clusterName"`
	ExternalDNSName          string `yaml:"externalDNSName"`
	KeyName                  string `yaml:"keyName"`
	Region                   string `yaml:"region"`
	AvailabilityZone         string `yaml:"availabilityZone"`
	ReleaseChannel           string `yaml:"releaseChannel"`
	ControllerInstanceType   string `yaml:"controllerInstanceType"`
	ControllerRootVolumeSize int    `yaml:"controllerRootVolumeSize"`
	WorkerCount              int    `yaml:"workerCount"`
	WorkerInstanceType       string `yaml:"workerInstanceType"`
	WorkerRootVolumeSize     int    `yaml:"workerRootVolumeSize"`
	WorkerSpotPrice          string `yaml:"workerSpotPrice"`
	VPCCIDR                  string `yaml:"vpcCIDR"`
	InstanceCIDR             string `yaml:"instanceCIDR"`
	ControllerIP             string `yaml:"controllerIP"`
	PodCIDR                  string `yaml:"podCIDR"`
	ServiceCIDR              string `yaml:"serviceCIDR"`
	KubernetesServiceIP      string `yaml:"kubernetesServiceIP"`
	DNSServiceIP             string `yaml:"dnsServiceIP"`
	K8sVer                   string `yaml:"kubernetesVersion"`
}

func (c Cluster) Config(tlsConfig *RawTLSAssets) (*Config, error) {
	config := Config{Cluster: c}
	config.ETCDEndpoints = fmt.Sprintf("http://%s:2379", c.ControllerIP)
	config.APIServers = fmt.Sprintf("http://%s:8080", c.ControllerIP)
	config.SecureAPIServers = fmt.Sprintf("https://%s:443", c.ControllerIP)
	config.APIServerEndpoint = fmt.Sprintf("https://%s", c.ExternalDNSName)

	var err error
	if config.AMI, err = getAMI(config.Region, config.ReleaseChannel); err != nil {
		return nil, fmt.Errorf("failed getting AMI for config: %v", err)
	}

	compact, err := tlsConfig.Compact()
	if err != nil {
		return nil, fmt.Errorf("failed to compress TLS assets: %v", err)
	}
	config.TLSConfig = compact

	return &config, nil
}

type StackTemplateOptions struct {
	TLSAssetsDir          string
	ControllerTmplFile    string
	WorkerTmplFile        string
	StackTemplateTmplFile string
}

type stackConfig struct {
	*Config
	UserDataWorker     string
	UserDataController string
}

func execute(filename string, data interface{}, compress bool) (string, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(filename).Parse(string(raw))
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, data); err != nil {
		return "", err
	}
	if compress {
		return compressData(buff.Bytes())
	}
	return buff.String(), nil
}

func (c Cluster) stackConfig(opts StackTemplateOptions, compressUserData bool) (*stackConfig, error) {
	assets, err := ReadTLSAssets(opts.TLSAssetsDir)
	if err != nil {
		return nil, err
	}
	stackConfig := stackConfig{}

	if stackConfig.Config, err = c.Config(assets); err != nil {
		return nil, err
	}

	if stackConfig.UserDataWorker, err = execute(opts.WorkerTmplFile, stackConfig.Config, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render worker cloud config: %v", err)
	}
	if stackConfig.UserDataController, err = execute(opts.ControllerTmplFile, stackConfig.Config, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render controller cloud config: %v", err)
	}

	return &stackConfig, nil
}

func (c Cluster) ValidateUserData(opts StackTemplateOptions) error {
	stackConfig, err := c.stackConfig(opts, false)
	if err != nil {
		return err
	}

	errors := []string{}

	for _, userData := range []struct {
		Name    string
		Content string
	}{
		{
			Content: stackConfig.UserDataWorker,
			Name:    "UserDataWorker",
		},
		{
			Content: stackConfig.UserDataController,
			Name:    "UserDataController",
		},
	} {
		report, err := validate.Validate([]byte(userData.Content))

		if err != nil {
			errors = append(
				errors,
				fmt.Sprintf("cloud-config %s could not be parsed: %v",
					userData.Name,
					err,
				),
			)
			continue
		}

		for _, entry := range report.Entries() {
			errors = append(errors, fmt.Sprintf("%s: %+v", userData.Name, entry))
		}
	}

	if len(errors) > 0 {
		reportString := strings.Join(errors, "\n")
		return fmt.Errorf("cloud-config validation errors:\n%s\n", reportString)
	}

	return nil
}

func (c Cluster) RenderStackTemplate(opts StackTemplateOptions) ([]byte, error) {
	stackConfig, err := c.stackConfig(opts, true)
	if err != nil {
		return nil, err
	}

	rendered, err := execute(opts.StackTemplateTmplFile, stackConfig, false)
	if err != nil {
		return nil, err
	}
	// minify JSON
	var buff bytes.Buffer
	if err := json.Compact(&buff, []byte(rendered)); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

type Config struct {
	Cluster

	ETCDEndpoints     string
	APIServers        string
	SecureAPIServers  string
	APIServerEndpoint string
	AMI               string

	// Encoded TLS assets
	TLSConfig *CompactTLSAssets
}

func (cfg Cluster) valid() error {
	if cfg.ExternalDNSName == "" {
		return errors.New("externalDNSName must be set")
	}
	if cfg.KeyName == "" {
		return errors.New("keyName must be set")
	}
	if cfg.Region == "" {
		return errors.New("region must be set")
	}
	if cfg.AvailabilityZone == "" {
		return errors.New("availabilityZone must be set")
	}
	if cfg.ClusterName == "" {
		return errors.New("clusterName must be set")
	}

	_, vpcNet, err := net.ParseCIDR(cfg.VPCCIDR)
	if err != nil {
		return fmt.Errorf("invalid vpcCIDR: %v", err)
	}

	instancesNetIP, instancesNet, err := net.ParseCIDR(cfg.InstanceCIDR)
	if err != nil {
		return fmt.Errorf("invalid instanceCIDR: %v", err)
	}
	if !vpcNet.Contains(instancesNetIP) {
		return fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s)",
			cfg.VPCCIDR,
			cfg.InstanceCIDR,
		)
	}

	controllerIPAddr := net.ParseIP(cfg.ControllerIP)
	if controllerIPAddr == nil {
		return fmt.Errorf("invalid controllerIP: %s", cfg.ControllerIP)
	}
	if !instancesNet.Contains(controllerIPAddr) {
		return fmt.Errorf("instanceCIDR (%s) does not contain controllerIP (%s)",
			cfg.InstanceCIDR,
			cfg.ControllerIP,
		)
	}

	podNetIP, podNet, err := net.ParseCIDR(cfg.PodCIDR)
	if err != nil {
		return fmt.Errorf("invalid podCIDR: %v", err)
	}
	if vpcNet.Contains(podNetIP) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with podCIDR (%s)", cfg.VPCCIDR, cfg.PodCIDR)
	}

	serviceNetIP, serviceNet, err := net.ParseCIDR(cfg.ServiceCIDR)
	if err != nil {
		return fmt.Errorf("invalid serviceCIDR: %v", err)
	}
	if vpcNet.Contains(serviceNetIP) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with serviceCIDR (%s)", cfg.VPCCIDR, cfg.ServiceCIDR)
	}
	if podNet.Contains(serviceNetIP) || serviceNet.Contains(podNetIP) {
		return fmt.Errorf("serviceCIDR (%s) overlaps with podCIDR (%s)", cfg.ServiceCIDR, cfg.PodCIDR)
	}

	kubernetesServiceIPAddr := net.ParseIP(cfg.KubernetesServiceIP)
	if kubernetesServiceIPAddr == nil {
		return fmt.Errorf("Invalid kubernetesServiceIP: %s", cfg.KubernetesServiceIP)
	}
	if !serviceNet.Contains(kubernetesServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain kubernetesServiceIP (%s)", cfg.ServiceCIDR, cfg.KubernetesServiceIP)
	}

	dnsServiceIPAddr := net.ParseIP(cfg.DNSServiceIP)
	if dnsServiceIPAddr == nil {
		return fmt.Errorf("Invalid dnsServiceIP: %s", cfg.DNSServiceIP)
	}
	if !serviceNet.Contains(dnsServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain dnsServiceIP (%s)", cfg.ServiceCIDR, cfg.DNSServiceIP)
	}

	return nil
}
