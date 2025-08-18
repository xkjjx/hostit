package main

import (
	"fmt"
	"log"
	"os"
	"sort"
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

	dnsOptions := map[string]string{
		"A": "AWS",
	}
	fmt.Println("What DNS Provider do you want to use?")
	var dnsKeys []string
	for k := range dnsOptions {
		dnsKeys = append(dnsKeys, k)
	}
	sort.Strings(dnsKeys)
	for _, k := range dnsKeys {
		fmt.Printf("[%s]\t%s\n", k, dnsOptions[k])
	}
	var enteredDnsProvider string
	fmt.Scanln(&enteredDnsProvider)
	if _, ok := dnsOptions[enteredDnsProvider]; !ok {
		log.Fatalf("DNS provider not supported")
	}

	objectStorageOptions := map[string]string{
		"G": "Github",
		"S": "S3",
	}
	fmt.Println("What object storage platform do you want to use?")
	var objectKeys []string
	for k := range objectStorageOptions {
		objectKeys = append(objectKeys, k)
	}
	sort.Strings(objectKeys)
	for _, k := range objectKeys {
		fmt.Printf("[%s]\t%s\n", k, objectStorageOptions[k])
	}
	var enteredObjectStorageProvider string
	fmt.Scanln(&enteredObjectStorageProvider)
	if _, ok := objectStorageOptions[enteredObjectStorageProvider]; !ok {
		log.Fatalf("Object storage provider not supported")
	}
	var objectStorageProviderManager ObjectStorageProviderManager
	var err error
	if enteredObjectStorageProvider == "G" {
		objectStorageProviderManager, err = NewGithubObjectStorageProviderManager(fullDomainName, folderName)
	} else {
		objectStorageProviderManager, err = NewS3ObjectStorageProviderManager(fullDomainName, folderName)
	}
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	err = objectStorageProviderManager.InstantiateClient()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	namespaceGood, err := objectStorageProviderManager.VerifyNamespace()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	if !namespaceGood {
		log.Fatalf("Repository with name %s already exists", fullDomainName)
	}
	err = objectStorageProviderManager.CreateStorageInstance()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	err = objectStorageProviderManager.UploadFilesToNewInstance()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	fmt.Println("Finished all uploads")
	err = objectStorageProviderManager.CreateAvailableDomain()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	var resourceRecordSets []*types.ResourceRecordSet
	resourceRecordSets, err = objectStorageProviderManager.GetRequiredDnsRecords()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	var dnsProviderManager DnsProviderManager
	if enteredDnsProvider == "A" {
		dnsProviderManager, err = NewAwsDnsProviderManager(fullDomainName, domainName, resourceRecordSets)
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
	fmt.Println("Domain name properly configured in DNS provider")
	err = dnsProviderManager.AddSubdomainRecords()
	if err != nil {
		log.Fatalf("Error: %s", err.Error())
	}
	fmt.Println("Subdomain records added")
	fmt.Printf("Website should now be accessible at https://%s\n", fullDomainName)
}
