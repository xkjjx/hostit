package main

type DnsProviderManager interface {
	InstantiateClient() error
	VerifyDomainExists() (bool, error)
	AddSubdomainRecords() error
}
