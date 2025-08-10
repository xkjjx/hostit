package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: hostit <file_name> <domain_name>")
		os.Exit(1)
	}

	fmt.Println("What DNS Provider do you want to use?")
	fmt.Println("[A]\tAWS")

	supportedDnsProviders := NewSet[string]()
	supportedDnsProviders.Add("A")
	var enteredDnsProvider string
	fmt.Scanln(&enteredDnsProvider)
	if !supportedDnsProviders.Contains(enteredDnsProvider) {
		fmt.Println("DNS provider not supported")
		os.Exit(1)
	}

	fmt.Println("What object storage platform do you want to use?")
	fmt.Println("[G]\tGithub")

	supportedObjectStorageProviders := NewSet[string]()
	supportedObjectStorageProviders.Add("G")
	var enteredObjectStorageProvider string
	fmt.Scanln(&enteredObjectStorageProvider)
	if !supportedObjectStorageProviders.Contains(enteredObjectStorageProvider) {
		fmt.Println("DNS provider not supported")
		os.Exit(1)
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
			fmt.Println("Error: " + err.Error())
			os.Exit(1)
		}
	}
	isCredentialsValid, err := dnsProviderManager.VerifyCredentials()
	if err != nil {
		fmt.Println("Error: " + err.Error())
		os.Exit(1)
	}
	if !isCredentialsValid {
		fmt.Println("DNS provider credentials are not valid")
		os.Exit(1)
	}
	isDomainAvailable, err := dnsProviderManager.VerifyDomainExists()
	if err != nil {
		fmt.Println("Error: " + err.Error())
		os.Exit(1)
	}
	if !isDomainAvailable {
		fmt.Println("Hosted zone for base domain not found")
		os.Exit(1)
	}
	fmt.Println("Domain name properly configured in DNS provider")
	err = dnsProviderManager.AddSubdomainRecord()
	if err != nil {
		fmt.Println("Error: " + err.Error())
		os.Exit(1)
	}
	fmt.Println("Subdomain record added")
}
