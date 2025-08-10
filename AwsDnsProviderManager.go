package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AwsDnsProviderManager struct {
	subdomainName string
	route53Client *route53.Client
	resourceRecordSet *types.ResourceRecordSet
}

func (awsDnsProviderManager *AwsDnsProviderManager) InstantiateClient() error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return errors.New("issue with getting credentials")
	}
	stsClient := sts.NewFromConfig(cfg)
	result, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return errors.New("issue with user information")
	}
	fmt.Printf("Using AWS Account %s\n", *result.Account)
	awsDnsProviderManager.route53Client = route53.NewFromConfig(cfg)
	return nil
}

func (awsDnsProviderManager *AwsDnsProviderManager) VerifyCredentials() (bool, error) {
	err := awsDnsProviderManager.InstantiateClient()
	if err != nil {
		return false, nil
	} else {
		return true, nil
	}
}

func (awsDnsProviderManager *AwsDnsProviderManager) VerifyDomainExists() (bool, error) {
	s := awsDnsProviderManager.subdomainName
	lastDot := strings.LastIndex(s, ".")
	secondLastDot := strings.LastIndex(s[:lastDot], ".")
	domainName := s[secondLastDot+1:]
	if awsDnsProviderManager.route53Client == nil {
		return false, errors.New("route53 client not initialized")
	}
	maxItemsInOutput := int32(100)
	listHostedZonesByNameInput := route53.ListHostedZonesByNameInput{
		DNSName: &domainName,
		MaxItems: &maxItemsInOutput,
	}
	hostedZonesOutput, err := awsDnsProviderManager.route53Client.ListHostedZonesByName(context.Background(), &listHostedZonesByNameInput)
	if err != nil {
		return false, err
	}
	for _, hz := range hostedZonesOutput.HostedZones {
		if hz.Name == nil {
			continue
		}
		name := strings.TrimSuffix(*hz.Name, ".")
		if strings.EqualFold(name, domainName) {
			return true, nil
		}
	}
	return false, nil
}

func (awsDnsProviderManager *AwsDnsProviderManager) AddSubdomainRecord() error {
	return nil
}

func NewAwsDnsProviderManager(subdomainName string, resourceRecordSet *types.ResourceRecordSet) (*AwsDnsProviderManager, error) {
	if !strings.Contains(subdomainName, ".") {
		return nil, errors.New("not a proper subdomain name")
	}
	if !strings.Contains(subdomainName[:strings.LastIndex(subdomainName, ".")], ".") {
		return nil, errors.New("not a proper subdomain name")
	}
	return &AwsDnsProviderManager{
		subdomainName: subdomainName,
		route53Client: nil,
		resourceRecordSet: resourceRecordSet,
	}, nil
}
