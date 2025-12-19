package main

import (
	"fmt"
	"os"

	"github.com/arturoeanton/go-xml/xml"
)

func main() {
	// Router simple de comandos
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
	case "demo":
		target := "all"
		if len(args) > 0 {
			target = args[0]
		}
		RunDemos(target)
	default:
		fmt.Printf("Error: Comando desconocido '%s'\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("r2/xml - The Enterprise XML Swiss Army Knife")
	fmt.Println("Uso: r2xml [comando] [argumentos]")
	fmt.Println("\nComandos:")
	fmt.Println("  fmt   <file>          : Formatear/Embellecer XML (Pretty Print)")
	fmt.Println("  json  <file>          : Convertir XML a JSON")
	fmt.Println("  csv   <file> --path=X : Convertir lista XML a CSV (Flatten)")
	fmt.Println("  query <file> <xpath>  : Ejecutar consulta XPath")
	fmt.Println("  soap  <config.json>   : Ejecutar request SOAP desde definición JSON")
	fmt.Println("  call  [flags]         : Ejecutar request SOAP rápido con parámetros")
	fmt.Println("        --url=... --action=... --ns=... --auth=wsse --user=... --pass=...")
	fmt.Println("        --data=\"Key=Val\" --data=\"Nested/Key=Val\"")
	fmt.Println("  demo                  : Ejecutar demos internas")
	fmt.Println("  demo [nombre]         : Ejecutar demo específica")

	fmt.Println("\nExample call")
	fmt.Println("  r2xml call --url=http://www.dneonline.com/calculator.asmx --action=Add --ns=http://tempuri.org/ --data=\"intA=3\" --data=\"intB=4\"")
}
