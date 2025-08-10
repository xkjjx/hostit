package main

import (
	"fmt"
	"log"
	"os"
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
	var dnsProviderManager DnsProviderManager
	var err error
	if enteredDnsProvider == "A" {
		dnsProviderManager, err = NewAwsDnsProviderManager(os.Args[1], nil)
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
	err = githubObjectStorageProviderManager.UploadFiles()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
}
