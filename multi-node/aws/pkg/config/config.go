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
	"unicode/utf8"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"

	"github.com/coreos/coreos-cloudinit/config/validate"
	yaml "gopkg.in/yaml.v2"
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
		ControllerIP:             "10.0.0.50",
		PodCIDR:                  "10.2.0.0/16",
		ServiceCIDR:              "10.3.0.0/24",
		DNSServiceIP:             "10.3.0.10",
		K8sVer:                   "v1.2.4_coreos.1",
		HyperkubeImageRepo:       "quay.io/coreos/hyperkube",
		ControllerInstanceType:   "m3.medium",
		ControllerRootVolumeSize: 30,
		WorkerCount:              1,
		WorkerInstanceType:       "m3.medium",
		WorkerRootVolumeSize:     30,
		CreateRecordSet:          false,
		RecordSetTTL:             300,
		Subnets:                  []Subnet{},
	}
}

func ClusterFromFile(filename string) (*Cluster, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c, err := ClusterFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("file %s: %v", filename, err)
	}

	return c, nil
}

// ClusterFromBytes Necessary for unit tests, which store configs as hardcoded strings
func ClusterFromBytes(data []byte) (*Cluster, error) {
	c := newDefaultCluster()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("failed to parse cluster: %v", err)
	}

	// HostedZone needs to end with a '.', amazon will not append it for you.
	// as it will with RecordSets
	c.HostedZone = WithTrailingDot(c.HostedZone)

	// If the user specified no subnets, we assume that a single AZ configuration with the default instanceCIDR is demanded
	if len(c.Subnets) == 0 && c.InstanceCIDR == "" {
		c.InstanceCIDR = "10.0.0.0/24"
	}

	if err := c.valid(); err != nil {
		return nil, fmt.Errorf("invalid cluster: %v", err)
	}

	// For backward-compatibility
	if len(c.Subnets) == 0 {
		c.Subnets = []Subnet{
			{
				AvailabilityZone: c.AvailabilityZone,
				InstanceCIDR:     c.InstanceCIDR,
			},
		}
	}
	return c, nil
}

type Cluster struct {
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
	VPCID                    string            `yaml:"vpcId"`
	RouteTableID             string            `yaml:"routeTableId"`
	VPCCIDR                  string            `yaml:"vpcCIDR"`
	InstanceCIDR             string            `yaml:"instanceCIDR"`
	ControllerIP             string            `yaml:"controllerIP"`
	PodCIDR                  string            `yaml:"podCIDR"`
	ServiceCIDR              string            `yaml:"serviceCIDR"`
	DNSServiceIP             string            `yaml:"dnsServiceIP"`
	K8sVer                   string            `yaml:"kubernetesVersion"`
	HyperkubeImageRepo       string            `yaml:"hyperkubeImageRepo"`
	KMSKeyARN                string            `yaml:"kmsKeyArn"`
	CreateRecordSet          bool              `yaml:"createRecordSet"`
	RecordSetTTL             int               `yaml:"recordSetTTL"`
	HostedZone               string            `yaml:"hostedZone"`
	StackTags                map[string]string `yaml:"stackTags"`
	UseCalico                bool              `yaml:"useCalico"`
	Subnets                  []Subnet          `yaml:"subnets"`
}

type Subnet struct {
	AvailabilityZone string `yaml:"availabilityZone"`
	InstanceCIDR     string `yaml:"instanceCIDR"`
}

const (
	vpcLogicalName = "VPC"
)

var supportedReleaseChannels = map[string]bool{
	"alpha":  true,
	"beta":   true,
	"stable": false,
}

func (c Cluster) Config() (*Config, error) {
	config := Config{Cluster: c}
	config.ETCDEndpoints = fmt.Sprintf("http://%s:2379", c.ControllerIP)
	config.APIServers = fmt.Sprintf("http://%s:8080", c.ControllerIP)
	config.SecureAPIServers = fmt.Sprintf("https://%s:443", c.ControllerIP)
	config.APIServerEndpoint = fmt.Sprintf("https://%s", c.ExternalDNSName)
	if config.UseCalico {
		config.K8sNetworkPlugin = "cni"
	}

	var err error
	if config.AMI, err = getAMI(config.Region, config.ReleaseChannel); err != nil {
		return nil, fmt.Errorf("failed getting AMI for config: %v", err)
	}

	//Set logical name constants
	config.VPCLogicalName = vpcLogicalName

	//Set reference strings

	//Assume VPC does not exist, reference by logical name
	config.VPCRef = fmt.Sprintf(`{ "Ref" : %q }`, config.VPCLogicalName)
	if config.VPCID != "" {
		//This means this VPC already exists, and we can reference it directly by ID
		config.VPCRef = fmt.Sprintf("%q", config.VPCID)
	}

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
	UserDataWorker        string
	UserDataController    string
	ControllerSubnetIndex int
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

	if stackConfig.Config, err = c.Config(); err != nil {
		return nil, err
	}

	awsConfig := aws.NewConfig().
		WithRegion(stackConfig.Config.Region).
		WithCredentialsChainVerboseErrors(true)

	kmsSvc := kms.New(session.New(awsConfig))

	compactAssets, err := assets.compact(stackConfig.Config, kmsSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to compress TLS assets: %v", err)
	}

	stackConfig.Config.TLSConfig = compactAssets

	controllerIPAddr := net.ParseIP(stackConfig.ControllerIP)
	if controllerIPAddr == nil {
		return nil, fmt.Errorf("invalid controllerIP: %s", stackConfig.ControllerIP)
	}
	controllerSubnetFound := false
	for i, subnet := range stackConfig.Subnets {
		_, instanceCIDR, err := net.ParseCIDR(subnet.InstanceCIDR)
		if err != nil {
			return nil, fmt.Errorf("invalid instanceCIDR: %v", err)
		}
		if instanceCIDR.Contains(controllerIPAddr) {
			stackConfig.ControllerSubnetIndex = i
			controllerSubnetFound = true
		}
	}
	if !controllerSubnetFound {
		return nil, fmt.Errorf("Fail-fast occurred possibly because of a bug: ControllerSubnetIndex couldn't be determined for subnets (%v) and controllerIP (%v)", stackConfig.Subnets, stackConfig.ControllerIP)
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

	//Use unmarshal function to do syntax validation
	renderedBytes := []byte(rendered)
	var jsonHolder map[string]interface{}
	if err := json.Unmarshal(renderedBytes, &jsonHolder); err != nil {
		syntaxError, ok := err.(*json.SyntaxError)
		if ok {
			contextString := getContextString(renderedBytes, int(syntaxError.Offset), 3)
			return nil, fmt.Errorf("%v:\njson syntax error (offset=%d), in this region:\n-------\n%s\n-------\n", err, syntaxError.Offset, contextString)
		}
		return nil, err
	}

	// minify JSON
	var buff bytes.Buffer
	if err := json.Compact(&buff, renderedBytes); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func getContextString(buf []byte, offset, lineCount int) string {

	linesSeen := 0
	var leftLimit int
	for leftLimit = offset; leftLimit > 0 && linesSeen <= lineCount; leftLimit-- {
		if buf[leftLimit] == '\n' {
			linesSeen++
		}
	}

	linesSeen = 0
	var rightLimit int
	for rightLimit = offset + 1; rightLimit < len(buf) && linesSeen <= lineCount; rightLimit++ {
		if buf[rightLimit] == '\n' {
			linesSeen++
		}
	}

	return string(buf[leftLimit:rightLimit])
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

	//Logical names of dynamic resources
	VPCLogicalName string

	//Reference strings for dynamic resources
	VPCRef string

	K8sNetworkPlugin string
}

func (c Cluster) valid() error {
	if c.ExternalDNSName == "" {
		return errors.New("externalDNSName must be set")
	}

	releaseChannelSupported := supportedReleaseChannels[c.ReleaseChannel]
	if !releaseChannelSupported {
		return fmt.Errorf("releaseChannel %s is not supported", c.ReleaseChannel)
	}

	if c.CreateRecordSet {
		if c.HostedZone == "" {
			return errors.New("hostedZone cannot be blank when createRecordSet is true")
		}
		if c.RecordSetTTL < 1 {
			return errors.New("TTL must be at least 1 second")
		}
		if !isSubdomain(c.ExternalDNSName, c.HostedZone) {
			return fmt.Errorf("%s is not a subdomain of %s",
				c.ExternalDNSName,
				c.HostedZone)
		}
	} else {
		if c.RecordSetTTL != newDefaultCluster().RecordSetTTL {
			return errors.New(
				"recordSetTTL should not be modified when createRecordSet is false",
			)
		}
	}
	if c.KeyName == "" {
		return errors.New("keyName must be set")
	}
	if c.Region == "" {
		return errors.New("region must be set")
	}
	if c.ClusterName == "" {
		return errors.New("clusterName must be set")
	}
	if c.KMSKeyARN == "" {
		return errors.New("kmsKeyArn must be set")
	}

	if c.VPCID == "" && c.RouteTableID != "" {
		return errors.New("vpcId must be specified if routeTableId is specified")
	}

	_, vpcNet, err := net.ParseCIDR(c.VPCCIDR)
	if err != nil {
		return fmt.Errorf("invalid vpcCIDR: %v", err)
	}

	controllerIPAddr := net.ParseIP(c.ControllerIP)
	if controllerIPAddr == nil {
		return fmt.Errorf("invalid controllerIP: %s", c.ControllerIP)
	}

	if len(c.Subnets) == 0 {
		if c.AvailabilityZone == "" {
			return fmt.Errorf("availabilityZone must be set")
		}
		_, instanceCIDR, err := net.ParseCIDR(c.InstanceCIDR)
		if err != nil {
			return fmt.Errorf("invalid instanceCIDR: %v", err)
		}
		if !vpcNet.Contains(instanceCIDR.IP) {
			return fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s)",
				c.VPCCIDR,
				c.InstanceCIDR,
			)
		}
		if !instanceCIDR.Contains(controllerIPAddr) {
			return fmt.Errorf("instanceCIDR (%s) does not contain controllerIP (%s)",
				c.InstanceCIDR,
				c.ControllerIP,
			)
		}
	} else {
		if c.InstanceCIDR != "" {
			return fmt.Errorf("The top-level instanceCIDR(%s) must be empty when subnets are specified", c.InstanceCIDR)
		}
		if c.AvailabilityZone != "" {
			return fmt.Errorf("The top-level availabilityZone(%s) must be empty when subnets are specified", c.AvailabilityZone)
		}

		var instanceCIDRs = make([]*net.IPNet, 0)
		for i, subnet := range c.Subnets {
			if subnet.AvailabilityZone == "" {
				return fmt.Errorf("availabilityZone must be set for subnet #%d", i)
			}
			_, instanceCIDR, err := net.ParseCIDR(subnet.InstanceCIDR)
			if err != nil {
				return fmt.Errorf("invalid instanceCIDR for subnet #%d: %v", i, err)
			}
			instanceCIDRs = append(instanceCIDRs, instanceCIDR)
			if !vpcNet.Contains(instanceCIDR.IP) {
				return fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s) for subnet #%d",
					c.VPCCIDR,
					c.InstanceCIDR,
					i,
				)
			}
		}

		controllerInstanceCidrExists := false
		for _, a := range instanceCIDRs {
			if a.Contains(controllerIPAddr) {
				controllerInstanceCidrExists = true
			}
		}
		if !controllerInstanceCidrExists {
			return fmt.Errorf("No instanceCIDRs in Subnets (%v) contain controllerIP (%s)",
				instanceCIDRs,
				c.ControllerIP,
			)
		}

		for i, a := range instanceCIDRs {
			for j, b := range instanceCIDRs[i+1:] {
				if i > 0 && cidrOverlap(a, b) {
					return fmt.Errorf("CIDR of subnet %d (%s) overlaps with CIDR of subnet %d (%s)", i, a, j, b)
				}
			}
		}
	}

	_, podNet, err := net.ParseCIDR(c.PodCIDR)
	if err != nil {
		return fmt.Errorf("invalid podCIDR: %v", err)
	}

	_, serviceNet, err := net.ParseCIDR(c.ServiceCIDR)
	if err != nil {
		return fmt.Errorf("invalid serviceCIDR: %v", err)
	}
	if cidrOverlap(serviceNet, vpcNet) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with serviceCIDR (%s)", c.VPCCIDR, c.ServiceCIDR)
	}
	if cidrOverlap(podNet, vpcNet) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with podCIDR (%s)", c.VPCCIDR, c.PodCIDR)
	}
	if cidrOverlap(serviceNet, podNet) {
		return fmt.Errorf("serviceCIDR (%s) overlaps with podCIDR (%s)", c.ServiceCIDR, c.PodCIDR)
	}

	kubernetesServiceIPAddr := incrementIP(serviceNet.IP)
	if !serviceNet.Contains(kubernetesServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain kubernetesServiceIP (%s)", c.ServiceCIDR, kubernetesServiceIPAddr)
	}

	dnsServiceIPAddr := net.ParseIP(c.DNSServiceIP)
	if dnsServiceIPAddr == nil {
		return fmt.Errorf("Invalid dnsServiceIP: %s", c.DNSServiceIP)
	}
	if !serviceNet.Contains(dnsServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain dnsServiceIP (%s)", c.ServiceCIDR, c.DNSServiceIP)
	}

	if dnsServiceIPAddr.Equal(kubernetesServiceIPAddr) {
		return fmt.Errorf("dnsServiceIp conflicts with kubernetesServiceIp (%s)", dnsServiceIPAddr)
	}

	return nil
}

/*
Validates the an existing VPC and it's existing subnets do not conflict with this
cluster configuration
*/
func (c *Cluster) ValidateExistingVPC(existingVPCCIDR string, existingSubnetCIDRS []string) error {

	_, existingVPC, err := net.ParseCIDR(existingVPCCIDR)
	if err != nil {
		return fmt.Errorf("error parsing existing vpc cidr %s : %v", existingVPCCIDR, err)
	}

	existingSubnets := make([]*net.IPNet, len(existingSubnetCIDRS))
	for i, existingSubnetCIDR := range existingSubnetCIDRS {
		_, existingSubnets[i], err = net.ParseCIDR(existingSubnetCIDR)
		if err != nil {
			return fmt.Errorf(
				"error parsing existing subnet cidr %s : %v",
				existingSubnetCIDR,
				err,
			)
		}
	}
	_, instanceNet, err := net.ParseCIDR(c.InstanceCIDR)
	if len(c.Subnets) == 0 && err != nil {
		return fmt.Errorf("error parsing instances cidr %s : %v", c.InstanceCIDR, err)
	}
	_, vpcNet, err := net.ParseCIDR(c.VPCCIDR)
	if err != nil {
		return fmt.Errorf("error parsing vpc cidr %s: %v", c.VPCCIDR, err)
	}

	//Verify that existing vpc CIDR matches declared vpc CIDR
	if vpcNet.String() != existingVPC.String() {
		return fmt.Errorf(
			"declared vpcCidr %s does not match existing vpc cidr %s",
			vpcNet,
			existingVPC,
		)
	}

	//Loop through all existing subnets in the VPC and look for conflicting CIDRS
	for _, existingSubnet := range existingSubnets {
		if cidrOverlap(instanceNet, existingSubnet) {
			return fmt.Errorf(
				"instance cidr (%s) conflicts with existing subnet cidr=%s",
				instanceNet,
				existingSubnet,
			)
		}
	}

	return nil
}

//Return next IP address in network range
func incrementIP(netIP net.IP) net.IP {
	ip := make(net.IP, len(netIP))
	copy(ip, netIP)

	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}

	return ip
}

//Does the address space of these networks "a" and "b" overlap?
func cidrOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

func WithTrailingDot(s string) string {
	if s == "" {
		return s
	}
	lastRune, _ := utf8.DecodeLastRuneInString(s)
	if lastRune != rune('.') {
		return s + "."
	}
	return s
}

func isSubdomain(sub, parent string) bool {
	sub, parent = WithTrailingDot(sub), WithTrailingDot(parent)
	subParts, parentParts := strings.Split(sub, "."), strings.Split(parent, ".")

	if len(parentParts) > len(subParts) {
		return false
	}

	subSuffixes := subParts[len(subParts)-len(parentParts):]

	if len(subSuffixes) != len(parentParts) {
		return false
	}
	for i := range subSuffixes {
		if subSuffixes[i] != parentParts[i] {
			return false
		}
	}
	return true
}
