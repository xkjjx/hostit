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
	subdomainName      string
	domainName         string
	route53Client      *route53.Client
	resourceRecordSets []*types.ResourceRecordSet
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
	fmt.Printf("Using AWS Account %s for DNS provider\n", *result.Account)
	awsDnsProviderManager.route53Client = route53.NewFromConfig(cfg)
	return nil
}

func (awsDnsProviderManager *AwsDnsProviderManager) VerifyDomainExists() (bool, error) {
	if awsDnsProviderManager.route53Client == nil {
		return false, errors.New("route53 client not initialized")
	}
	maxItemsInOutput := int32(100)
	listHostedZonesByNameInput := route53.ListHostedZonesByNameInput{
		DNSName:  &awsDnsProviderManager.domainName,
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
		if strings.EqualFold(name, awsDnsProviderManager.domainName) {
			return true, nil
		}
	}
	return false, nil
}

func (awsDnsProviderManager *AwsDnsProviderManager) AddSubdomainRecords() error {
	if awsDnsProviderManager.route53Client == nil {
		return errors.New("route53 client not initialized")
	}
	if len(awsDnsProviderManager.resourceRecordSets) == 0 {
		return errors.New("no resource record sets provided")
	}

	maxItemsInOutput := int32(100)
	listHostedZonesByNameInput := route53.ListHostedZonesByNameInput{
		DNSName:  &awsDnsProviderManager.domainName,
		MaxItems: &maxItemsInOutput,
	}
	hostedZonesOutput, err := awsDnsProviderManager.route53Client.ListHostedZonesByName(context.Background(), &listHostedZonesByNameInput)
	if err != nil {
		return err
	}

	var hostedZoneId string
	for _, hz := range hostedZonesOutput.HostedZones {
		if hz.Name == nil || hz.Id == nil {
			continue
		}
		name := strings.TrimSuffix(*hz.Name, ".")
		if strings.EqualFold(name, awsDnsProviderManager.domainName) {
			hostedZoneId = *hz.Id
			break
		}
	}
	if hostedZoneId == "" {
		return errors.New("hosted zone for domain not found")
	}

	// Normalize hosted zone id (it may come prefixed with "/hostedzone/")
	hostedZoneId = strings.TrimPrefix(hostedZoneId, "/hostedzone/")

	changes := make([]types.Change, 0, len(awsDnsProviderManager.resourceRecordSets))
	for _, rrset := range awsDnsProviderManager.resourceRecordSets {
		if rrset == nil {
			continue
		}
		changes = append(changes, types.Change{
			Action:            types.ChangeActionUpsert,
			ResourceRecordSet: rrset,
		})
	}
	if len(changes) == 0 {
		return errors.New("no valid resource record sets to apply")
	}

	changeInput := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &hostedZoneId,
		ChangeBatch: &types.ChangeBatch{
			Changes: changes,
		},
	}

	_, err = awsDnsProviderManager.route53Client.ChangeResourceRecordSets(context.Background(), changeInput)
	if err != nil {
		return err
	}

	return nil
}

func NewAwsDnsProviderManager(subdomainName string, domainName string, resourceRecordSets []*types.ResourceRecordSet) (*AwsDnsProviderManager, error) {
	if !strings.Contains(subdomainName, ".") {
		return nil, errors.New("not a proper subdomain name")
	}
	if !strings.Contains(subdomainName[:strings.LastIndex(subdomainName, ".")], ".") {
		return nil, errors.New("not a proper subdomain name")
	}
	return &AwsDnsProviderManager{
		subdomainName:      subdomainName,
		domainName:         domainName,
		route53Client:      nil,
		resourceRecordSets: resourceRecordSets,
	}, nil
}
