package main

import (
	"fmt"
	"os"

	"github.com/arturoeanton/go-xml/xml"
)

func main() {
	args := os.Args[1:]

	// 1. Modo Demo
	if len(args) > 0 && args[0] == "--demo" {
		target := "all"
		if len(args) > 1 {
			target = args[1]
		}
		RunDemos(target)
		return
	}

	// 2. Modo CLI Tool (Query)
	// Ejemplo: echo "..." | go run main.go query "path"
	xml.RunCLI()

	// 3. Ayuda por defecto
	fmt.Println("r2/xml CLI")
	fmt.Println("Uso:")
	fmt.Println("  --demo [name]   : Ejecuta demos (basic, stream_r, hooks, etc)")
	fmt.Println("  query \"path\"    : Ejecuta consulta (Sintaxis tipo XPath) sobre stdin")
}
