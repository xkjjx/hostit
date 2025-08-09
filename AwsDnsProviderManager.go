package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AwsDnsProviderManager struct {
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

func NewAwsDnsProviderManager() *AwsDnsProviderManager {
	return &AwsDnsProviderManager{
		route53Client: nil,
	}
}
