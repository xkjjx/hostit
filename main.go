package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: hostit <file_name> <domain_name>")
	}

	log.Println("What DNS Provider do you want to use?")
	log.Println("[A]\tAWS")

	supportedDnsProviders := NewSet[string]()
	supportedDnsProviders.Add("A")
	var enteredDnsProvider string
	fmt.Scanln(&enteredDnsProvider)
	if !supportedDnsProviders.Contains(enteredDnsProvider) {
		log.Fatalf("DNS provider not supported")
	}

	log.Println("What object storage platform do you want to use?")
	log.Println("[G]\tGithub")

	supportedObjectStorageProviders := NewSet[string]()
	supportedObjectStorageProviders.Add("G")
	var enteredObjectStorageProvider string
	fmt.Scanln(&enteredObjectStorageProvider)
	if !supportedObjectStorageProviders.Contains(enteredObjectStorageProvider) {
		log.Fatalf("Object storage provider not supported")
	}

	var err error

	githubObjectStorageProviderManager, err := NewGithubObjectStorageProviderManager(os.Args[1], os.Args[2])
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	err = githubObjectStorageProviderManager.InstantiateClient()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	namespaceGood, err := githubObjectStorageProviderManager.VerifyNamespace()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	if !namespaceGood {
		log.Fatalf("Repository with name %s already exists", os.Args[1])
	}
	err = githubObjectStorageProviderManager.CreateStorageInstance()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	err = githubObjectStorageProviderManager.UploadFilesToNewInstance()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	err = githubObjectStorageProviderManager.CreateAvailableDomain()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	var resourceRecordSet *types.ResourceRecordSet
	resourceRecordSet, err = githubObjectStorageProviderManager.GetRequiredDnsRecord()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	var dnsProviderManager DnsProviderManager
	if enteredDnsProvider == "A" {
		dnsProviderManager, err = NewAwsDnsProviderManager(os.Args[1], resourceRecordSet)
		if err != nil {
			log.Fatalf("Error: %s", err.Error())
		}
	}
	err = dnsProviderManager.InstantiateClient()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	isDomainAvailable, err := dnsProviderManager.VerifyDomainExists()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	if !isDomainAvailable {
		log.Fatalf("Hosted zone for base domain not found")
	}
	log.Println("Domain name properly configured in DNS provider")
	err = dnsProviderManager.AddSubdomainRecord()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	log.Println("Subdomain record added")
}
