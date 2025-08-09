package main

import (
	"fmt"
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
		dnsProviderManager, err = NewAwsDnsProviderManager(os.Args[1])
		if err != nil {
			fmt.Println("Error when initializing Route53 client")
			os.Exit(1)
		}
	}
	dnsProviderManager.VerifyCredentials()
}
