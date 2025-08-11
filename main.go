package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: hostit <domain_name> <folder_name>")
	}

	fullDomainName, folderName := os.Args[1], os.Args[2]
	temporaryDomainName := fullDomainName
	var domainName string
	if strings.LastIndex(fullDomainName, ".") == strings.Index(fullDomainName, ".") || strings.LastIndex(fullDomainName, ".") == -1 {
		log.Fatalf("invalid domain name")
	}
	temporaryDomainName = temporaryDomainName[strings.Index(temporaryDomainName, ".")+1:]
	if strings.Index(temporaryDomainName, ".") == strings.LastIndex(temporaryDomainName, ".") {
		domainName = temporaryDomainName
	} else {
		fmt.Println("Multiple possible base domain names detected - please select which one you'd like to use")
		var possibleDomainNames []string
		for {
			possibleDomainNames = append(possibleDomainNames, temporaryDomainName)
			if strings.LastIndex(temporaryDomainName, ".") == strings.Index(temporaryDomainName, ".") {
				break
			}
			temporaryDomainName = temporaryDomainName[strings.Index(temporaryDomainName, ".")+1:]
		}
		for index, value := range possibleDomainNames {
			fmt.Printf("[%d]\t%s\n", index+1, value)
		}
		var numChosen int
		_, err := fmt.Scanln(&numChosen)
		if err != nil || numChosen > len(possibleDomainNames) {
			log.Fatalf("invalid option chosen")
		}
		domainName = possibleDomainNames[numChosen-1]
	}
	fmt.Printf("Using base domain name %s\n", domainName)
	fmt.Println("What DNS Provider do you want to use?")
	fmt.Println("[A]\tAWS")
	supportedDnsProviders := NewSet[string]()
	supportedDnsProviders.Add("A")
	var enteredDnsProvider string
	fmt.Scanln(&enteredDnsProvider)
	if !supportedDnsProviders.Contains(enteredDnsProvider) {
		log.Fatalf("DNS provider not supported")
	}
	fmt.Println("What object storage platform do you want to use?")
	fmt.Println("[G]\tGithub")
	supportedObjectStorageProviders := NewSet[string]()
	supportedObjectStorageProviders.Add("G")
	var enteredObjectStorageProvider string
	fmt.Scanln(&enteredObjectStorageProvider)
	if !supportedObjectStorageProviders.Contains(enteredObjectStorageProvider) {
		log.Fatalf("Object storage provider not supported")
	}
	var err error
	githubObjectStorageProviderManager, err := NewGithubObjectStorageProviderManager(fullDomainName, folderName)
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
		log.Fatalf("Repository with name %s already exists", fullDomainName)
	}
	err = githubObjectStorageProviderManager.CreateStorageInstance()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	err = githubObjectStorageProviderManager.UploadFilesToNewInstance()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	fmt.Println("Finished all uploads")
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
		dnsProviderManager, err = NewAwsDnsProviderManager(fullDomainName, domainName, resourceRecordSet)
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
	fmt.Println("Subdomain record added")
	fmt.Printf("Website should now be accessible at https://%s\n", fullDomainName)
}
