package main

type DnsProviderManager interface {
	VerifyCredentials() (bool, error)
	VerifyDomainExists() (bool, error)
	AddSubdomainRecord() error
}
