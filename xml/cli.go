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
