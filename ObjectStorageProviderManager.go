package main

import "github.com/aws/aws-sdk-go-v2/service/route53/types"

type ObjectStorageProviderManager interface {
	InstantiateClient() error
	VerifyNamespace() (bool, error)
	CreateStorageInstance() error
	UploadFilesToNewInstance() error
	CreateAvailableDomain() error
	GetRequiredDnsRecords() ([]*types.ResourceRecordSet, error)
	FinalizeHttps() error
}
