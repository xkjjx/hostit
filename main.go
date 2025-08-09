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
	if enteredDnsProvider == "A" {
		dnsProviderManager = NewAwsDnsProviderManager()
	}
	dnsProviderManager.VerifyCredentials()
}
