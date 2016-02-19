package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/blobutil"
	yaml "gopkg.in/yaml.v2"
)

const (
	credentialsDir = "./credentials"
	userDataDir    = "./userdata"
)

func NewDefaultConfig() *Config {
	return &Config{
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
		Tags:                     make(map[string]string),

		TLSConfig:     newTLSConfig(),
		UserData:      newUserDataConfig(),
		KubeConfig:    &blobutil.NamedBuffer{Name: "kubeconfig"},
		StackTemplate: &blobutil.NamedBuffer{Name: "stack-template.json"},
	}
}

type Config struct {
	ClusterName              string            `yaml:"clusterName"`
	ExternalDNSName          string            `yaml:"externalDNSName"`
	KeyName                  string            `yaml:"keyName"`
	Region                   string            `yaml:"region"`
	AvailabilityZone         string            `yaml:"availabilityZone"`
	ReleaseChannel           string            `yaml:"releaseChannel"`
	ControllerInstanceType   string            `yaml:"controllerInstanceType"`
	ControllerRootVolumeSize int               `yaml:"controllerRootVolumeSize"`
	WorkerCount              int               `yaml:"workerCount"`
	WorkerInstanceType       string            `yaml:"workerInstanceType"`
	WorkerRootVolumeSize     int               `yaml:"workerRootVolumeSize"`
	WorkerSpotPrice          string            `yaml:"workerSpotPrice"`
	VPCCIDR                  string            `yaml:"vpcCIDR"`
	InstanceCIDR             string            `yaml:"instanceCIDR"`
	ControllerIP             string            `yaml:"controllerIP"`
	PodCIDR                  string            `yaml:"podCIDR"`
	ServiceCIDR              string            `yaml:"serviceCIDR"`
	KubernetesServiceIP      string            `yaml:"kubernetesServiceIP"`
	DNSServiceIP             string            `yaml:"dnsServiceIP"`
	K8sVer                   string            `yaml:"kubernetesVersion"`
	Tags                     map[string]string `yaml:"tags"`

	//Calculated fields
	APIServers        string `yaml:"-"`
	SecureAPIServers  string `yaml:"-"`
	ETCDEndpoints     string `yaml:"-"`
	APIServerEndpoint string `yaml:"-"`

	//Subconfig
	TLSConfig     *TLSConfig            `yaml:"-"`
	UserData      *UserDataConfig       `yaml:"-"`
	KubeConfig    *blobutil.NamedBuffer `yaml:"-"`
	StackTemplate *blobutil.NamedBuffer `yaml:"-"`
}

func (cfg *Config) valid() error {
	if cfg.ExternalDNSName == "" {
		return errors.New("externalDNSName must be set")
	}
	if cfg.KeyName == "" {
		return errors.New("keyName must be set")
	}
	if cfg.Region == "" {
		return errors.New("region must be set")
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

func (cfg *Config) GenerateDefaultAssets() error {
	if err := cfg.TLSConfig.generateAllTLS(cfg); err != nil {
		return err
	}

	if err := cfg.UserData.generateDefaultConfigs(); err != nil {
		return err
	}

	kubeConfigBuffer := bytes.NewBufferString(defaultKubeConfigTemplate)
	if _, err := cfg.KubeConfig.ReadFrom(kubeConfigBuffer); err != nil {
		return err
	}

	defaultStackTemplate, err := generateDefaultStackTemplate()
	if err != nil {
		return err
	}

	stackTemplateBuffer := bytes.NewBufferString(defaultStackTemplate)
	if _, err := cfg.StackTemplate.ReadFrom(stackTemplateBuffer); err != nil {
		return err
	}

	return nil
}

func (cfg *Config) WriteAssetsToFiles() error {
	gitIgnorePath := "./.gitignore"
	if err := ioutil.WriteFile(gitIgnorePath, []byte("/credentials/*.pem\n"), 0600); err != nil {
		return fmt.Errorf("Error writing .gitignore file %s: %v", gitIgnorePath, err)
	}

	for _, dir := range []string{credentialsDir, userDataDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return fmt.Errorf("Error creating directory %s : %v", dir, err)
		}
	}

	if err := cfg.TLSConfig.buffers.WriteToFiles(credentialsDir); err != nil {
		return err
	}

	if err := cfg.UserData.buffers.WriteToFiles(userDataDir); err != nil {
		return err
	}

	if err := cfg.KubeConfig.WriteToFile(credentialsDir); err != nil {
		return err
	}

	if err := cfg.StackTemplate.WriteToFile("./"); err != nil {
		return err
	}

	return nil
}

func (cfg *Config) ReadAssetsFromFiles() error {
	if err := cfg.TLSConfig.buffers.ReadFromFiles(credentialsDir); err != nil {
		return err
	}

	if err := cfg.UserData.buffers.ReadFromFiles(userDataDir); err != nil {
		return err
	}

	if err := cfg.KubeConfig.ReadFromFile(credentialsDir); err != nil {
		return err
	}

	if err := cfg.StackTemplate.ReadFromFile("./"); err != nil {
		return err
	}

	return nil
}

func (cfg *Config) TemplateAndEncodeAssets() error {

	//Template kubeconfig
	if err := cfg.KubeConfig.Template(cfg); err != nil {
		return err
	}

	//Template and encode tls assets
	if err := cfg.TLSConfig.buffers.TemplateBuffers(cfg); err != nil {
		return err
	}
	if err := cfg.TLSConfig.buffers.EncodeBuffers(); err != nil {
		return err
	}

	//Template and encode userdata assets
	if err := cfg.UserData.buffers.TemplateBuffers(cfg); err != nil {
		return err
	}

	if err := cfg.UserData.validate(); err != nil {
		return fmt.Errorf("user-data validation error: %s", err)
	}

	if err := cfg.UserData.buffers.EncodeBuffers(); err != nil {
		return err
	}

	//Template cloudformation stack
	if err := cfg.StackTemplate.Template(cfg); err != nil {
		return err
	}
	stackMap := map[string]interface{}{}

	if err := json.Unmarshal([]byte(cfg.StackTemplate.String()), &stackMap); err != nil {
		return fmt.Errorf("Error unmarshaling stack template: %v", err)
	}

	stackResources, ok := stackMap["Resources"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Error parsing stack template: 'Resources' key is not a dictionary")
	}

	for name, resource := range stackResources {
		properties, ok := resource.(map[string]interface{})["Properties"]
		if ok {
			propertiesMap, ok := properties.(map[string]interface{})
			if !ok {
				return fmt.Errorf("Error parsing stack template: 'Properties' of %s is not a dictionary", name)
			}
			tags, ok := propertiesMap["Tags"]
			if ok {
				propagateAtLaunchFound := false
				propagateAtLaunchValue := ""
				tagsArray := tags.([]interface{})
				if firstTag, ok := tagsArray[0].(map[string]interface{}); ok {
					if p, ok := firstTag["PropagateAtLaunch"].(string); ok {
						propagateAtLaunchFound = true
						propagateAtLaunchValue = p
					}
				}
				for key, value := range cfg.Tags {
					tag := make(map[string]string)
					tag["Key"] = key
					tag["Value"] = value
					if propagateAtLaunchFound {
						tag["PropagateAtLaunch"] = propagateAtLaunchValue
					}
					tagsArray = append(tagsArray, tag)
				}
				propertiesMap["Tags"] = tagsArray
			}
		}
	}

	output, err := json.MarshalIndent(&stackMap, "", "  ")
	if err != nil {
		return fmt.Errorf("Error marshalling stack template: %v", err)
	}
	cfg.StackTemplate.Reset()
	cfg.StackTemplate.Write(output)

	return nil
}

func NewConfigFromFile(loc string) (*Config, error) {
	d, err := ioutil.ReadFile(loc)
	if err != nil {
		return nil, fmt.Errorf("failed reading config file: %v", err)
	}

	return newConfigFromBytes(d)
}

func newConfigFromBytes(d []byte) (*Config, error) {
	out := NewDefaultConfig()
	if err := yaml.Unmarshal(d, out); err != nil {
		return nil, fmt.Errorf("failed decoding config file: %v", err)
	}

	if err := out.valid(); err != nil {
		return nil, fmt.Errorf("config file invalid: %v", err)
	}

	//TODO: this will look different once we support multiple controllers
	out.ETCDEndpoints = fmt.Sprintf("http://%s:2379", out.ControllerIP)
	out.APIServers = fmt.Sprintf("http://%s:8080", out.ControllerIP)
	out.SecureAPIServers = fmt.Sprintf("https://%s:443", out.ControllerIP)
	out.APIServerEndpoint = fmt.Sprintf("https://%s", out.ExternalDNSName)

	return out, nil
}
