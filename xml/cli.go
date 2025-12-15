package xml

import (
	"encoding/json"
	"fmt"
	"os"
)

// ============================================================================
// 6. CLI TOOL HELPER (Feature 4: Herramienta de Consola)
// ============================================================================

// RunCLI convierte tu programa en una herramienta de consola básica.
// Uso: go run main.go query "users/user[0]/name" < data.xml
func RunCLI() {
	if len(os.Args) < 2 {
		return
	} // No es modo CLI

	cmd := os.Args[1]
	if cmd == "query" && len(os.Args) >= 3 {
		path := os.Args[2]
		// Leer de STDIN
		m, err := MapXML(os.Stdin, ForceArray("item")) // Default config
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parseando XML: %v\n", err)
			os.Exit(1)
		}

		res, err := QueryAll(m, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error query: %v\n", err)
			os.Exit(1)
		}

		// Output JSON para facilitar uso en terminal
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(res)
		os.Exit(0)
	}
}
