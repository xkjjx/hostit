package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AwsDnsProviderManager struct {
	subdomainName string
	route53Client *route53.Client
}

func (awsDnsProviderManager AwsDnsProviderManager) InstantiateClient() error {
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

func (awsDnsProviderManager AwsDnsProviderManager) VerifyCredentials() (bool, error) {
	err := awsDnsProviderManager.InstantiateClient()
	if err != nil {
		return false, nil
	} else {
		return true, nil
	}
}

func (awsDnsProviderManager AwsDnsProviderManager) VerifyDomainExists() (bool, error) {
	return true, nil
}

func (awsDnsProviderManager AwsDnsProviderManager) AddSubdomainRecord() error {
	return nil
}

func NewAwsDnsProviderManager(subdomainName string) (*AwsDnsProviderManager, error) {
	if !strings.Contains(subdomainName, ".") {
		return nil, errors.New("not a proper domain name")
	}
	subdomainName = subdomainName[:strings.LastIndex(subdomainName, ".")]
	if !strings.Contains(subdomainName, ".") {
		return nil, errors.New("not a proper domain name")
	}
	return &AwsDnsProviderManager{
		subdomainName: subdomainName,
		route53Client: nil,
	}, nil
}
