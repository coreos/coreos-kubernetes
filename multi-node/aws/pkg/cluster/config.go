package cluster

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"

	"gopkg.in/yaml.v2"
)

const (
	DefaultVPCCIDR             = "10.0.0.0/16"
	DefaultInstanceCIDRA       = "10.0.10.0/24"
	DefaultInstanceCIDRB       = "10.0.20.0/24"
	DefaultPodCIDR             = "10.2.0.0/16"
	DefaultServiceCIDR         = "10.3.0.0/24"
	DefaultKubernetesServiceIP = "10.3.0.1"
	DefaultDNSServiceIP        = "10.3.0.10"
)

var (
	DefaultClusterName = "kubernetes"
)

type Config struct {
	ClusterName              string `yaml:"clusterName"`
	ExternalDNSName          string `yaml:"externalDNSName"`
	KeyName                  string `yaml:"keyName"`
	Region                   string `yaml:"region"`
	AvailabilityZone         string `yaml:"availabilityZone"`
	ArtifactURL              string `yaml:"artifactURL"`
	ReleaseChannel           string `yaml:"releaseChannel"`
	ControllerCount          int    `yaml:"controllerCount"`
	ControllerInstanceType   string `yaml:"controllerInstanceType"`
	ControllerRootVolumeSize int    `yaml:"controllerRootVolumeSize"`
	WorkerCount              int    `yaml:"workerCount"`
	WorkerInstanceType       string `yaml:"workerInstanceType"`
	WorkerRootVolumeSize     int    `yaml:"workerRootVolumeSize"`
	VPCCIDR                  string `yaml:"vpcCIDR"`
	InstanceCIDRA            string `yaml:"instanceCIDRA"`
	InstanceCIDRB            string `yaml:"instanceCIDRB"`
	PodCIDR                  string `yaml:"podCIDR"`
	ServiceCIDR              string `yaml:"serviceCIDR"`
	KubernetesServiceIP      string `yaml:"kubernetesServiceIP"`
	DNSServiceIP             string `yaml:"dnsServiceIP"`
}

func (cfg *Config) Valid() error {
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
	if _, err := url.Parse(cfg.ArtifactURL); err != nil {
		return fmt.Errorf("invalid artifactURL: %v", err)
	}

	vpcCIDR := cfg.VPCCIDR
	if vpcCIDR == "" {
		vpcCIDR = DefaultVPCCIDR
	}
	_, vpcNet, err := net.ParseCIDR(vpcCIDR)
	if err != nil {
		return fmt.Errorf("invalid vpcCIDR: %v", err)
	}

	instanceCIDRA := cfg.InstanceCIDRA
	if instanceCIDRA == "" {
		instanceCIDRA = DefaultInstanceCIDRA
	}
	instancesNetIPA, _, err := net.ParseCIDR(instanceCIDRA)
	if err != nil {
		return fmt.Errorf("invalid instanceCIDR A: %v", err)
	}
	if !vpcNet.Contains(instancesNetIPA) {
		return fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s)",
			vpcCIDR,
			instanceCIDRA,
		)
	}

	instanceCIDRB := cfg.InstanceCIDRB
	if instanceCIDRB == "" {
		instanceCIDRB = DefaultInstanceCIDRB
	}
	instancesNetIPB, _, err := net.ParseCIDR(instanceCIDRB)
	if err != nil {
		return fmt.Errorf("invalid instanceCIDR B: %v", err)
	}
	if !vpcNet.Contains(instancesNetIPB) {
		return fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s)",
			vpcCIDR,
			instanceCIDRB,
		)
	}

	podCIDR := cfg.PodCIDR
	if podCIDR == "" {
		podCIDR = DefaultPodCIDR
	}
	podNetIP, podNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return fmt.Errorf("invalid podCIDR: %v", err)
	}
	if vpcNet.Contains(podNetIP) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with podCIDR (%s)", vpcCIDR, podCIDR)
	}

	serviceCIDR := cfg.ServiceCIDR
	if serviceCIDR == "" {
		serviceCIDR = DefaultServiceCIDR
	}
	serviceNetIP, serviceNet, err := net.ParseCIDR(serviceCIDR)
	if err != nil {
		return fmt.Errorf("invalid serviceCIDR: %v", err)
	}
	if vpcNet.Contains(serviceNetIP) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with serviceCIDR (%s)", vpcCIDR, serviceCIDR)
	}
	if podNet.Contains(serviceNetIP) || serviceNet.Contains(podNetIP) {
		return fmt.Errorf("serviceCIDR (%s) overlaps with podCIDR (%s)", serviceCIDR, podCIDR)
	}

	kubernetesServiceIP := cfg.KubernetesServiceIP
	if kubernetesServiceIP == "" {
		kubernetesServiceIP = DefaultKubernetesServiceIP
	}
	kubernetesServiceIPAddr := net.ParseIP(kubernetesServiceIP)
	if kubernetesServiceIPAddr == nil {
		return fmt.Errorf("Invalid kubernetesServiceIP: %s", kubernetesServiceIP)
	}
	if !serviceNet.Contains(kubernetesServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain kubernetesServiceIP (%s)", serviceCIDR, kubernetesServiceIP)
	}

	dnsServiceIP := cfg.DNSServiceIP
	if dnsServiceIP == "" {
		dnsServiceIP = DefaultDNSServiceIP
	}
	dnsServiceIPAddr := net.ParseIP(dnsServiceIP)
	if dnsServiceIPAddr == nil {
		return fmt.Errorf("Invalid dnsServiceIP: %s", dnsServiceIP)
	}
	if !serviceNet.Contains(dnsServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain dnsServiceIP (%s)", serviceCIDR, dnsServiceIP)
	}

	return nil
}

func DecodeConfigFromFile(out *Config, loc string) error {
	d, err := ioutil.ReadFile(loc)
	if err != nil {
		return fmt.Errorf("failed reading config file: %v", err)
	}

	if err = yaml.Unmarshal(d, &out); err != nil {
		return fmt.Errorf("failed decoding config file: %v", err)
	}

	if err = out.Valid(); err != nil {
		return fmt.Errorf("config file invalid: %v", err)
	}

	return nil
}

func NewDefaultConfig(ver string) *Config {
	return &Config{
		ClusterName: "kubernetes",
		ArtifactURL: DefaultArtifactURL(ver),
	}
}

func DefaultArtifactURL(ver string) string {
	return fmt.Sprintf("https://coreos-kubernetes.s3.amazonaws.com/%s", ver)
}
