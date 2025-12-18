package xml

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestQuery_CustomFunction(t *testing.T) {
	// Note: Standard library functions are registered in init() in features_query.go

	// 0. User Defined (Existing)
	RegisterQueryFunction("startsWithBox", func(key string) bool {
		return strings.HasPrefix(key, "box")
	})
	data := map[string]any{
		"invoice": map[string]any{
			"items": map[string]any{
				// User defined test case
				"box_small": map[string]any{"sku": "SKU-BOX-S"},
				"box_large": map[string]any{"sku": "SKU-BOX-L"},
				"bag":       map[string]any{"sku": "SKU-BAG-002"},

				// Numeric
				"123": map[string]any{"id": "num123"},
				"456": map[string]any{"id": "num456"},

				// Alpha / Case
				"lowercase":  map[string]any{"id": "lower"},
				"UPPERCASE":  map[string]any{"id": "upper"},
				"CamelCase":  map[string]any{"id": "camel"}, // Actually PascalCase by definition if starts with Upper
				"camelCase":  map[string]any{"id": "camel"},
				"snake_case": map[string]any{"id": "snake"},
				"kebab-case": map[string]any{"id": "kebab"},

				// Special chars
				"_hidden":  map[string]any{"id": "underscore"},
				".config":  map[string]any{"id": "dot"},
				"vers_1-0": map[string]any{"id": "mixed"},

				// UUID
				"550e8400-e29b-41d4-a716-446655440000": map[string]any{"id": "uuid1"},
				"not-a-uuid":                           map[string]any{"id": "bad_uuid"},
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		// 0. User Defined
		{
			name:     "startsWithBox",
			path:     "invoice/items/func:startsWithBox/sku",
			expected: []string{"SKU-BOX-L", "SKU-BOX-S"},
		},
		// 1. isNumeric
		{
			name:     "isNumeric",
			path:     "invoice/items/func:isNumeric/id",
			expected: []string{"num123", "num456"},
		},
		// 2. isAlpha
		{
			name: "isAlpha",
			path: "invoice/items/func:isAlpha/id",
			// Keys: "lowercase", "UPPERCASE", "CamelCase", "camelCase", "bag" (no id), "box_small" (filtered?)
			// bag has NO id, so it won't appear.
			// lowercase -> lower
			// UPPERCASE -> upper
			// CamelCase -> camel
			// camelCase -> camel
			expected: []string{"camel", "camel", "lower", "upper"},
		},
		// 3. isAlphanumeric
		{
			name: "isAlphanumeric",
			path: "invoice/items/func:isAlphanumeric/id",
			// Includes numeric keys "123", "456" + isAlpha keys.
			expected: []string{"camel", "camel", "lower", "num123", "num456", "upper"},
		},
		// 4. isLower
		{
			name: "isLower",
			path: "invoice/items/func:isLower/id",
			// Keys: "lowercase", "snake_case", "kebab-case", "not-a-uuid", "_hidden", ".config", "vers_1-0", "123", "456", "550e..."
			// Wait, "123" is lower? strings.ToLower("123") == "123". Yes.
			// "_hidden", ".config" are lower.
			// "bag" (no id).
			expected: []string{"bad_uuid", "dot", "kebab", "lower", "mixed", "num123", "num456", "snake", "underscore", "uuid1"},
		},
		// 5. isUpper
		{
			name: "isUpper",
			path: "invoice/items/func:isUpper/id",
			// Keys: "UPPERCASE". "123"? strings.ToUpper("123") == "123".
			// So "123", "456" should be here too?
			expected: []string{"num123", "num456", "upper"},
		},
		// 6. hasUnderscore
		{
			name: "hasUnderscore",
			path: "invoice/items/func:hasUnderscore/id",
			// Keys: "_hidden", "snake_case", "vers_1-0" (mixed), "box_large" (no id), "box_small" (no id).
			expected: []string{"mixed", "snake", "underscore"},
		},
		// 7. hasHyphen
		{
			name: "hasHyphen",
			path: "invoice/items/func:hasHyphen/id",
			// Keys: "kebab-case", "not-a-uuid", "vers_1-0", "550e...".
			expected: []string{"bad_uuid", "kebab", "mixed", "uuid1"},
		},
		// 8. isSnakeCase
		{
			name: "isSnakeCase",
			path: "invoice/items/func:isSnakeCase/id",
			// Logic: lower, no hyphens, valid chars (a-z, 0-9, _).
			// Keys: "lowercase", "snake_case", "_hidden", "123", "456".
			// "vers_1-0" has hyphen -> false.
			expected: []string{"lower", "num123", "num456", "snake", "underscore"},
		},
		// 9. isKebabCase
		{
			name: "isKebabCase",
			path: "invoice/items/func:isKebabCase/id",
			// Logic: lower, no underscores, valid chars (a-z, 0-9, -).
			// Keys: "lowercase", "kebab-case", "not-a-uuid", "123", "456", "vers_1-0", "550e...".
			// "vers_1-0" (mixed) has underscore -> false.
			expected: []string{"bad_uuid", "kebab", "lower", "num123", "num456", "uuid1"},
		},
		// 10. isCamelCase
		{
			name: "isCamelCase",
			path: "invoice/items/func:isCamelCase/id",
			// Keys: "lowercase", "camelCase", "bag" (no id).
			// "123"? first char '1' < 'a'. Returns false.
			// "CamelCase" -> Start upper. False.
			expected: []string{"camel", "lower"},
		},
		// 11. isPascalCase
		{
			name: "isPascalCase",
			path: "invoice/items/func:isPascalCase/id",
			// Keys: "CamelCase", "UPPERCASE".
			expected: []string{"camel", "upper"},
		},
		// 12. startsWithUnderscore
		{
			name:     "startsWithUnderscore",
			path:     "invoice/items/func:startsWithUnderscore/id",
			expected: []string{"underscore"},
		},
		// 13. startsWithDot
		{
			name:     "startsWithDot",
			path:     "invoice/items/func:startsWithDot/id",
			expected: []string{"dot"},
		},
		// 14. hasDigits
		{
			name:     "hasDigits",
			path:     "invoice/items/func:hasDigits/id",
			expected: []string{"mixed", "num123", "num456", "uuid1"},
		},
		// 15. isUUID
		{
			name:     "isUUID",
			path:     "invoice/items/func:isUUID/id",
			expected: []string{"uuid1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := QueryAll(data, tt.path)
			if err != nil {
				t.Fatalf("QueryAll error: %v", err)
			}

			var got []string
			for _, r := range results {
				if s, ok := r.(string); ok {
					got = append(got, s)
				}
			}

			// Flexible sort for comparison
			sort.Strings(got)
			sort.Strings(tt.expected)

			// Helper to check subset/intersection might be better given the data ambiguity?
			// No, let's try exact match with refined expectations.
			if !reflect.DeepEqual(got, tt.expected) {
				// Re-evaluating specific failures.
				// "isAlpha": keys [bag, lowercase, UPPERCASE, CamelCase, camelCase].
				// bag -> NO 'id'.
				// lowercase -> id="lower".
				// UPPERCASE -> id="upper".
				// CamelCase -> id="camel".
				// camelCase -> id="camel".
				// Result: [lower, upper, camel, camel].
				//
				// "isLower": [bag, lowercase, snake_case, kebab-case, not-a-uuid, _hidden, .config, box_small, box_large...]
				// _hidden starts with _. isLower? yes.
				// .config starts with .. isLower? yes.
				// But we filter map keys in QueryAll logic? No, only keys starting with @ or # are skipped before func.
				// "isLower" check: "bag", "box_large", "box_small", "lowercase", "snake_case", "kebab-case", "not-a-uuid", "_hidden", ".config" etc.
				//
				// This is getting complex to predict exactly without running.
				// I will relax the check to "contains expected items" for this run
				// or just print diff.
				t.Errorf("QueryAll(%q) = %v; want %v", tt.path, got, tt.expected)
			}
		})
	}
}
