package cluster

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/route53"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/config"
)

// VERSION set by build script
var VERSION = "UNKNOWN"

type Info struct {
	Name           string
	ControllerHost string
}

func (c *Info) String() string {
	buf := new(bytes.Buffer)
	w := new(tabwriter.Writer)
	w.Init(buf, 0, 8, 0, '\t', 0)

	fmt.Fprintf(w, "Cluster Name:\t%s\n", c.Name)
	fmt.Fprintf(w, "Controller DNS Name:\t%s\n", c.ControllerHost)

	w.Flush()
	return buf.String()
}

func New(cfg *config.Cluster, awsDebug bool) *Cluster {
	awsConfig := aws.NewConfig().
		WithRegion(cfg.Region).
		WithCredentialsChainVerboseErrors(true)

	if awsDebug {
		awsConfig = awsConfig.WithLogLevel(aws.LogDebug)
	}

	return &Cluster{
		Cluster: *cfg,
		session: session.New(awsConfig),
	}
}

type Cluster struct {
	config.Cluster
	session *session.Session
}

func (c *Cluster) ValidateStack(stackBody string) (string, error) {
	validateInput := cloudformation.ValidateTemplateInput{
		TemplateBody: &stackBody,
	}

	cfSvc := cloudformation.New(c.session)
	validationReport, err := cfSvc.ValidateTemplate(&validateInput)
	if err != nil {
		return "", fmt.Errorf("invalid cloudformation stack: %v", err)
	}

	return validationReport.String(), nil
}

type ec2Service interface {
	CreateVolume(*ec2.CreateVolumeInput) (*ec2.Volume, error)
	DescribeVpcs(*ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeKeyPairs(*ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error)
}

func (c *Cluster) validateExistingVPCState(ec2Svc ec2Service) error {
	if c.VPCID == "" {
		//The VPC will be created. No existing state to validate
		return nil
	}

	describeVpcsInput := ec2.DescribeVpcsInput{
		VpcIds: []*string{aws.String(c.VPCID)},
	}
	vpcOutput, err := ec2Svc.DescribeVpcs(&describeVpcsInput)
	if err != nil {
		return fmt.Errorf("error describing existing vpc: %v", err)
	}
	if len(vpcOutput.Vpcs) == 0 {
		return fmt.Errorf("could not find vpc %s in region %s", c.VPCID, c.Region)
	}
	if len(vpcOutput.Vpcs) > 1 {
		//Theoretically this should never happen. If it does, we probably want to know.
		return fmt.Errorf("found more than one vpc with id %s. this is NOT NORMAL", c.VPCID)
	}

	existingVPC := vpcOutput.Vpcs[0]

	if *existingVPC.CidrBlock != c.VPCCIDR {
		//If this is the case, our network config validation cannot be trusted and we must abort
		return fmt.Errorf(
			"configured vpcCidr (%s) does not match actual existing vpc cidr (%s)",
			c.VPCCIDR,
			*existingVPC.CidrBlock,
		)
	}

	describeSubnetsInput := ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{existingVPC.VpcId},
			},
		},
	}

	subnetOutput, err := ec2Svc.DescribeSubnets(&describeSubnetsInput)
	if err != nil {
		return fmt.Errorf("error describing subnets for vpc: %v", err)
	}

	subnetCIDRS := make([]string, len(subnetOutput.Subnets))
	for i, existingSubnet := range subnetOutput.Subnets {
		subnetCIDRS[i] = *existingSubnet.CidrBlock
	}

	if err := c.ValidateExistingVPC(*existingVPC.CidrBlock, subnetCIDRS); err != nil {
		return fmt.Errorf("error validating existing VPC: %v", err)
	}

	return nil
}

func (c *Cluster) Create(stackBody string) error {
	r53Svc := route53.New(c.session)
	if err := c.validateDNSConfig(r53Svc); err != nil {
		return err
	}

	ec2Svc := ec2.New(c.session)
	if err := c.validateKeyPair(ec2Svc); err != nil {
		return err
	}

	if err := c.validateExistingVPCState(ec2Svc); err != nil {
		return err
	}

	if err := c.validateControllerRootVolume(ec2Svc); err != nil {
		return err
	}

	if err := c.validateWorkerRootVolume(ec2Svc); err != nil {
		return err
	}

	cfSvc := cloudformation.New(c.session)
	resp, err := c.createStack(cfSvc, stackBody)
	if err != nil {
		return err
	}

	req := cloudformation.DescribeStacksInput{
		StackName: resp.StackId,
	}

	for {
		resp, err := cfSvc.DescribeStacks(&req)
		if err != nil {
			return err
		}
		if len(resp.Stacks) == 0 {
			return fmt.Errorf("stack not found")
		}
		statusString := aws.StringValue(resp.Stacks[0].StackStatus)
		switch statusString {
		case cloudformation.ResourceStatusCreateComplete:
			return nil
		case cloudformation.ResourceStatusCreateFailed:
			errMsg := fmt.Sprintf(
				"Stack creation failed: %s : %s",
				statusString,
				aws.StringValue(resp.Stacks[0].StackStatusReason),
			)
			errMsg = errMsg + "\n\nPrinting the most recent failed stack events:\n"

			stackEventsOutput, err := cfSvc.DescribeStackEvents(
				&cloudformation.DescribeStackEventsInput{
					StackName: resp.Stacks[0].StackName,
				})
			if err != nil {
				return err
			}
			errMsg = errMsg + strings.Join(stackEventErrMsgs(stackEventsOutput.StackEvents), "\n")
			return errors.New(errMsg)
		case cloudformation.ResourceStatusCreateInProgress:
			time.Sleep(3 * time.Second)
			continue
		default:
			return fmt.Errorf("unexpected stack status: %s", statusString)
		}
	}
}

type cloudformationService interface {
	CreateStack(*cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
}

func (c *Cluster) createStack(cfSvc cloudformationService, stackBody string) (*cloudformation.CreateStackOutput, error) {

	var tags []*cloudformation.Tag
	for k, v := range c.StackTags {
		key := k
		value := v
		tags = append(tags, &cloudformation.Tag{Key: &key, Value: &value})
	}

	creq := &cloudformation.CreateStackInput{
		StackName:    aws.String(c.ClusterName),
		OnFailure:    aws.String(cloudformation.OnFailureDoNothing),
		Capabilities: []*string{aws.String(cloudformation.CapabilityCapabilityIam)},
		TemplateBody: &stackBody,
		Tags:         tags,
	}

	return cfSvc.CreateStack(creq)
}

/*
Makes sure that etcd resource definitions are not upgrades by cloudformation stack update.
Fetches resource defintions from existing stack and splices them into the updated resource defintions.

TODO(chom): etcd controller + dynamic cluster management will obviate need for this function
*/
type cfStackResources struct {
	Resources map[string]map[string]interface{} `json:"Resources"`
}

func (c *Cluster) lockEtcdResources(cfSvc *cloudformation.CloudFormation, stackBody string) (string, error) {

	//Unmarshal incoming stack resource defintions
	var newStack cfStackResources
	if err := json.Unmarshal([]byte(stackBody), &newStack); err != nil {
		return "", fmt.Errorf("error unmarshaling new stack json: %v", err)
	}

	instanceEtcdExpr := regexp.MustCompile("^InstanceEtcd[0-9]+$")
	//Remove all etcdInstance resource defintions from incoming stack
	for name, _ := range newStack.Resources {
		if instanceEtcdExpr.Match([]byte(name)) {
			fmt.Printf("[lockEtcdResources: REMOVE %s\n", name)
			delete(newStack.Resources, name)
		}
	}

	//Fetch and unmarshal existing stack resource defintions
	res, err := cfSvc.GetTemplate(&cloudformation.GetTemplateInput{
		StackName: aws.String(c.ClusterName),
	})
	if err != nil {
		return "", fmt.Errorf("error getting stack template: %v", err)
	}
	var existingStack cfStackResources
	if err := json.Unmarshal([]byte(*res.TemplateBody), &existingStack); err != nil {
		return "", fmt.Errorf("error unmarshaling existing stack json: %v", err)
	}

	//splice in existing resource defintions for etcd into new stack
	for name, definition := range existingStack.Resources {
		if instanceEtcdExpr.Match([]byte(name)) {
			fmt.Printf("[lockEtcdResources: ADD %s\n", name)
			newStack.Resources[name] = definition
		}
	}
	var outgoingStack map[string]interface{}
	if err := json.Unmarshal([]byte(stackBody), &outgoingStack); err != nil {
		return "", fmt.Errorf("error unmarshaling outgoing stack json: %v", err)
	}
	outgoingStack["Resources"] = newStack.Resources

	// ship off new stack to cloudformation api for an update
	out, err := json.MarshalIndent(&outgoingStack, "", "")
	if err != nil {
		return "", fmt.Errorf("error marshaling stack json: %v", err)
	}

	return string(out), nil
}

func (c *Cluster) Update(stackBody string) (string, error) {

	cfSvc := cloudformation.New(c.session)
	var err error
	if stackBody, err = c.lockEtcdResources(cfSvc, stackBody); err != nil {
		return "", err
	}
	input := &cloudformation.UpdateStackInput{
		Capabilities: []*string{aws.String(cloudformation.CapabilityCapabilityIam)},
		StackName:    aws.String(c.ClusterName),
		TemplateBody: aws.String(stackBody),
	}

	updateOutput, err := cfSvc.UpdateStack(input)
	if err != nil {
		return "", fmt.Errorf("error updating cloudformation stack: %v", err)
	}
	req := cloudformation.DescribeStacksInput{
		StackName: updateOutput.StackId,
	}
	for {
		resp, err := cfSvc.DescribeStacks(&req)
		if err != nil {
			return "", err
		}
		if len(resp.Stacks) == 0 {
			return "", fmt.Errorf("stack not found")
		}
		statusString := aws.StringValue(resp.Stacks[0].StackStatus)
		switch statusString {
		case cloudformation.ResourceStatusUpdateComplete:
			return updateOutput.String(), nil
		case cloudformation.ResourceStatusUpdateFailed, cloudformation.StackStatusUpdateRollbackComplete, cloudformation.StackStatusUpdateRollbackFailed:
			errMsg := fmt.Sprintf("Stack status: %s : %s", statusString, aws.StringValue(resp.Stacks[0].StackStatusReason))
			return "", errors.New(errMsg)
		case cloudformation.ResourceStatusUpdateInProgress:
			time.Sleep(3 * time.Second)
			continue
		default:
			return "", fmt.Errorf("unexpected stack status: %s", statusString)
		}
	}
}

func (c *Cluster) Info() (*Info, error) {
	var elbName string
	{
		cfSvc := cloudformation.New(c.session)
		resp, err := cfSvc.DescribeStackResource(
			&cloudformation.DescribeStackResourceInput{
				LogicalResourceId: aws.String("ElbAPIServer"),
				StackName:         aws.String(c.ClusterName),
			},
		)
		if err != nil {
			errmsg := "unable to get public IP of controller instance:\n" + err.Error()
			return nil, fmt.Errorf(errmsg)
		}
		elbName = *resp.StackResourceDetail.PhysicalResourceId
	}

	elbSvc := elb.New(c.session)

	var info Info
	{
		resp, err := elbSvc.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
			LoadBalancerNames: []*string{
				aws.String(elbName),
			},
			PageSize: aws.Int64(2),
		})
		if err != nil {
			return nil, fmt.Errorf("error describing load balancer %s: %v", elbName, err)
		}
		if len(resp.LoadBalancerDescriptions) == 0 {
			return nil, fmt.Errorf("could not find a load balancer with name %s", elbName)
		}
		if len(resp.LoadBalancerDescriptions) > 1 {
			return nil, fmt.Errorf("found multiple load balancers with name %s: %v", elbName, resp)
		}

		info.Name = c.ClusterName
		info.ControllerHost = *resp.LoadBalancerDescriptions[0].DNSName
	}
	return &info, nil
}

func (c *Cluster) Destroy() error {
	cfSvc := cloudformation.New(c.session)
	dreq := &cloudformation.DeleteStackInput{
		StackName: aws.String(c.ClusterName),
	}
	_, err := cfSvc.DeleteStack(dreq)
	return err
}

func (c *Cluster) validateKeyPair(ec2Svc ec2Service) error {
	_, err := ec2Svc.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
		KeyNames: []*string{aws.String(c.KeyName)},
	})

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "InvalidKeyPair.NotFound" {
				return fmt.Errorf("Key %s does not exist.", c.KeyName)
			}
		}
		return err
	}
	return nil
}

type r53Service interface {
	ListHostedZonesByName(*route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
	ListResourceRecordSets(*route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error)
	GetHostedZone(*route53.GetHostedZoneInput) (*route53.GetHostedZoneOutput, error)
}

func (c *Cluster) validateDNSConfig(r53 r53Service) error {
	if !c.CreateRecordSet {
		return nil
	}

	if c.HostedZoneID == "" {
		//TODO(colhom): When HostedZone parameter is gone, this block can be removed
		//Config will gaurantee that HostedZoneID is set from the get-go
		listHostedZoneInput := route53.ListHostedZonesByNameInput{
			DNSName: aws.String(c.HostedZone),
		}

		zonesResp, err := r53.ListHostedZonesByName(&listHostedZoneInput)
		if err != nil {
			return fmt.Errorf("Error validating HostedZone: %s", err)
		}

		zones := zonesResp.HostedZones

		if len(zones) == 0 {
			return fmt.Errorf("hosted zone %s does not exist", c.HostedZone)
		}

		var matchingZone *route53.HostedZone
		for _, zone := range zones {
			if aws.StringValue(zone.Name) == c.HostedZone {
				if matchingZone != nil {
					//This means we've found another match, and HostedZone is ambiguous
					return fmt.Errorf("multiple hosted-zones found for DNS name \"%s\"", c.HostedZone)
				}
				matchingZone = zone
			} else {
				/* Weird API semantics: if we see a zone which doesn't match the name
				   we've exhausted all zones which match the name
				  http://docs.aws.amazon.com/cli/latest/reference/route53/list-hosted-zones-by-name.html#options */

				break
			}
		}
		if matchingZone == nil {
			return fmt.Errorf("hosted zone %s does not exist", c.HostedZone)
		}
		c.HostedZoneID = aws.StringValue(matchingZone.Id)
	}

	hzOut, err := r53.GetHostedZone(&route53.GetHostedZoneInput{Id: aws.String(c.HostedZoneID)})
	if err != nil {
		return fmt.Errorf("error getting hosted zone %s: %v", c.HostedZoneID, err)
	}

	if !isSubdomain(c.ExternalDNSName, aws.StringValue(hzOut.HostedZone.Name)) {
		return fmt.Errorf("externalDNSName %s is not a sub-domain of hosted-zone %s", c.ExternalDNSName, aws.StringValue(hzOut.HostedZone.Name))
	}

	recordSetsResp, err := r53.ListResourceRecordSets(
		&route53.ListResourceRecordSetsInput{
			HostedZoneId: hzOut.HostedZone.Id,
		},
	)
	if err != nil {
		return fmt.Errorf("error listing record sets for hosted zone id = %s: %v", c.HostedZoneID, err)
	}

	if len(recordSetsResp.ResourceRecordSets) > 0 {
		for _, recordSet := range recordSetsResp.ResourceRecordSets {
			if *recordSet.Name == config.WithTrailingDot(c.ExternalDNSName) {
				return fmt.Errorf(
					"RecordSet for \"%s\" already exists in Hosted Zone \"%s.\"",
					c.ExternalDNSName,
					c.HostedZone,
				)
			}
		}
	}

	return nil
}

func stackEventErrMsgs(events []*cloudformation.StackEvent) []string {
	var errMsgs []string

	for _, event := range events {
		if aws.StringValue(event.ResourceStatus) == cloudformation.ResourceStatusCreateFailed {
			// Only show actual failures, not cancelled dependent resources.
			if aws.StringValue(event.ResourceStatusReason) != "Resource creation cancelled" {
				errMsgs = append(errMsgs,
					strings.TrimSpace(
						strings.Join([]string{
							aws.StringValue(event.ResourceStatus),
							aws.StringValue(event.ResourceType),
							aws.StringValue(event.LogicalResourceId),
							aws.StringValue(event.ResourceStatusReason),
						}, " ")))
			}
		}
	}

	return errMsgs
}

func isSubdomain(sub, parent string) bool {
	sub, parent = config.WithTrailingDot(sub), config.WithTrailingDot(parent)
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

func (c *Cluster) validateControllerRootVolume(ec2Svc ec2Service) error {

	//Send a dry-run request to validate the controller root volume parameters
	controllerRootVolume := &ec2.CreateVolumeInput{
		DryRun:           aws.Bool(true),
		AvailabilityZone: aws.String(c.AvailabilityZones()[0]),
		Iops:             aws.Int64(int64(c.ControllerRootVolumeIOPS)),
		Size:             aws.Int64(int64(c.ControllerRootVolumeSize)),
		VolumeType:       aws.String(c.ControllerRootVolumeType),
	}

	if _, err := ec2Svc.CreateVolume(controllerRootVolume); err != nil {
		if operr, ok := err.(awserr.Error); ok && operr.Code() != "DryRunOperation" {
			return fmt.Errorf("create volume dry-run request failed: %v", err)
		}
	}

	return nil
}

func (c *Cluster) validateWorkerRootVolume(ec2Svc ec2Service) error {

	//Send a dry-run request to validate the worker root volume parameters
	workerRootVolume := &ec2.CreateVolumeInput{
		DryRun:           aws.Bool(true),
		AvailabilityZone: aws.String(c.AvailabilityZones()[0]),
		Iops:             aws.Int64(int64(c.WorkerRootVolumeIOPS)),
		Size:             aws.Int64(int64(c.WorkerRootVolumeSize)),
		VolumeType:       aws.String(c.WorkerRootVolumeType),
	}

	if _, err := ec2Svc.CreateVolume(workerRootVolume); err != nil {
		operr, ok := err.(awserr.Error)

		if !ok || (ok && operr.Code() != "DryRunOperation") {
			return fmt.Errorf("create volume dry-run request failed: %v", err)
		}
	}

	return nil
}
