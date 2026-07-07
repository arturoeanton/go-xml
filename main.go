package main

import (
	"fmt"
	"os"

	"github.com/arturoeanton/go-xml/xml"
)

func main() {
	// Simple command router
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "fmt":
		xml.CliFormat(args)
	case "json":
		xml.CliToJson(args)
	case "csv":
		xml.CliToCsv(args)
	case "query":
		xml.CliQuery(args)
	case "soap":
		xml.CliSoap(args)
	case "call":
		xml.CliSoapQuick(args)
	case "wsdl":
		xml.CliWSDL(args)
	case "demo":
		target := "all"
		if len(args) > 0 {
			target = args[0]
		}
		RunDemos(target)
	default:
		fmt.Printf("Error: Unknown command '%s'\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("r2/xml - The Enterprise XML Swiss Army Knife")
	fmt.Println("Usage: r2xml [command] [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  fmt   <file>          : Format/Beautify XML (Pretty Print)")
	fmt.Println("  json  <file>          : Convert XML to JSON")
	fmt.Println("  csv   <file> --path=X : Convert XML list to CSV (Flatten)")
	fmt.Println("  query <file> <xpath>  : Run an XPath query")
	fmt.Println("  soap  <config.json>   : Execute a SOAP request from a JSON definition")
	fmt.Println("  call  [flags]         : Execute a quick SOAP request with parameters")
	fmt.Println("        --url=... --action=... --ns=... --auth=wsse --user=... --pass=...")
	fmt.Println("        --data=\"Key=Val\" --data=\"Nested/Key=Val\"")
	fmt.Println("        --wsdl=service.wsdl : use the WSDL to validate --action and set url/ns/soapAction")
	fmt.Println("  wsdl  <file.wsdl>     : List SOAP operations discovered in a WSDL")
	fmt.Println("  demo                  : Run built-in demos")
	fmt.Println("  demo [name]           : Run a specific demo")

	fmt.Println("\nExample call")
	fmt.Println("  r2xml call --url=http://www.dneonline.com/calculator.asmx --action=Add --ns=http://tempuri.org/ --data=\"intA=3\" --data=\"intB=4\"")
}
