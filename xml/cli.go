package xml

import (
	"encoding/json"
	"fmt"
	"os"
)

// ============================================================================
// 6. CLI TOOL HELPER (Feature: Console Utility)
// ============================================================================

// RunCLI transforms the program into a basic command-line utility.
// It detects command arguments, reads XML from STDIN, and outputs JSON.
//
// Usage:
//
//	cat data.xml | go run main.go query "users/user[0]/name"
func RunCLI() {
	if len(os.Args) < 2 {
		return // Not running in CLI mode, just return to main execution
	}

	cmd := os.Args[1]

	// 1. JSON Converter Mode
	// Usage:
	//   cat data.xml | go run main.go --json
	//   go run main.go data.xml --json
	//   go run main.go --json data.xml

	// Check if --json is in args
	hasJsonFlag := false
	filePath := ""

	for _, arg := range os.Args[1:] {
		if arg == "--json" {
			hasJsonFlag = true
		} else if arg != "query" && !os.IsNotExist(func() error { _, err := os.Stat(arg); return err }()) {
			// Simple heuristic: if it exists, it's a file
			filePath = arg
		}
	}

	if hasJsonFlag {
		var r *os.File = os.Stdin
		var err error

		if filePath != "" {
			r, err = os.Open(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file '%s': %v\n", filePath, err)
				os.Exit(1)
			}
			defer r.Close()
		}

		jsonBytes, err := ToJSON(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting to JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonBytes))
		os.Exit(0)
	}

	// 2. Query Mode
	if cmd == "query" && len(os.Args) >= 3 {
		path := os.Args[2]

		// Read from STDIN
		// Note: Uses ForceArray("item") as a sensible default for generic lists.
		m, err := MapXML(os.Stdin, ForceArray("item"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing XML: %v\n", err)
			os.Exit(1)
		}

		res, err := QueryAll(m, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query error: %v\n", err)
			os.Exit(1)
		}

		// Output result as indented JSON for easier consumption in the terminal (e.g., piping to jq)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(res)
		os.Exit(0)
	}
}
