package cluster

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func createStackAndWait(svc *cloudformation.CloudFormation, name, stackBody string) error {
	creq := &cloudformation.CreateStackInput{
		StackName:    aws.String(name),
		OnFailure:    aws.String("DO_NOTHING"),
		Capabilities: []*string{aws.String(cloudformation.CapabilityCapabilityIam)},
		TemplateBody: aws.String(stackBody),
	}

	resp, err := svc.CreateStack(creq)
	if err != nil {
		return err
	}

	if err := waitForStackCreateComplete(svc, aws.StringValue(resp.StackId)); err != nil {
		return err
	}

	return nil
}

func validateStack(svc *cloudformation.CloudFormation, stackBody string) (string, error) {

	input := &cloudformation.ValidateTemplateInput{
		TemplateBody: aws.String(stackBody),
	}

	validationReport, err := svc.ValidateTemplate(input)

	if err != nil {
		return "", fmt.Errorf("Invalid cloudformation stack: %v", err)
	}

	return validationReport.String(), err
}

func updateStack(svc *cloudformation.CloudFormation, stackName, stackBody string) (string, error) {

	input := &cloudformation.UpdateStackInput{
		Capabilities: []*string{aws.String(cloudformation.CapabilityCapabilityIam)},
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(stackBody),
	}

	updateOutput, err := svc.UpdateStack(input)

	if err != nil {
		return "", fmt.Errorf("Error updating cloudformation stack: %v", err)
	}

	return updateOutput.String(), waitForStackUpdateComplete(svc, *updateOutput.StackId)
}

func waitForStackUpdateComplete(svc *cloudformation.CloudFormation, stackID string) error {
	req := cloudformation.DescribeStacksInput{
		StackName: aws.String(stackID),
	}
	for {
		resp, err := svc.DescribeStacks(&req)
		if err != nil {
			return err
		}
		if len(resp.Stacks) == 0 {
			return fmt.Errorf("stack not found")
		}
		statusString := aws.StringValue(resp.Stacks[0].StackStatus)
		switch statusString {
		case cloudformation.ResourceStatusUpdateComplete:
			return nil
		case cloudformation.ResourceStatusUpdateFailed, cloudformation.StackStatusUpdateRollbackComplete, cloudformation.StackStatusUpdateRollbackFailed:
			errMsg := fmt.Sprintf("Stack status: %s : %s", statusString, aws.StringValue(resp.Stacks[0].StackStatusReason))
			return errors.New(errMsg)
		}
		time.Sleep(3 * time.Second)
	}
}

func waitForStackCreateComplete(svc *cloudformation.CloudFormation, stackID string) error {
	req := cloudformation.DescribeStacksInput{
		StackName: aws.String(stackID),
	}
	for {
		resp, err := svc.DescribeStacks(&req)
		if err != nil {
			return err
		}
		if len(resp.Stacks) == 0 {
			return fmt.Errorf("stack not found")
		}
		switch aws.StringValue(resp.Stacks[0].StackStatus) {
		case cloudformation.ResourceStatusCreateComplete:
			return nil
		case cloudformation.ResourceStatusCreateFailed:
			return errors.New(aws.StringValue(resp.Stacks[0].StackStatusReason))
		}
		time.Sleep(3 * time.Second)
	}
}

func getStackResources(svc *cloudformation.CloudFormation, stackID string) ([]cloudformation.StackResourceSummary, error) {
	resources := make([]cloudformation.StackResourceSummary, 0)
	req := cloudformation.ListStackResourcesInput{
		StackName: aws.String(stackID),
	}
	for {
		resp, err := svc.ListStackResources(&req)
		if err != nil {
			return nil, err
		}
		for _, s := range resp.StackResourceSummaries {
			resources = append(resources, *s)
		}
		req.NextToken = resp.NextToken
		if aws.StringValue(req.NextToken) == "" {
			break
		}
	}
	return resources, nil
}

func mapStackResourcesToClusterInfo(svc *ec2.EC2, resources []cloudformation.StackResourceSummary) (*ClusterInfo, error) {
	var info ClusterInfo
	for _, r := range resources {
		switch aws.StringValue(r.LogicalResourceId) {
		case "EIPController":
			if r.PhysicalResourceId != nil {
				info.ControllerIP = *r.PhysicalResourceId
			} else {
				return nil, fmt.Errorf("unable to get public IP of controller instance")
			}
		}
	}

	return &info, nil
}

func destroyStack(svc *cloudformation.CloudFormation, name string) error {
	dreq := &cloudformation.DeleteStackInput{
		StackName: aws.String(name),
	}
	_, err := svc.DeleteStack(dreq)
	return err
}
