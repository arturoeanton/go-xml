package xml

import (
	"testing"
)

// ============================================================================
// DATA FIXTURE (Mock Data)
// ============================================================================

func getQueryTestData() map[string]any {
	return map[string]any{
		"library": map[string]any{
			"@version": "1.0",
			"info":     "City Library", // Simplified primitive node
			"section": []any{
				// Section 0: Fiction
				map[string]any{
					"@name": "Fiction",
					"book": []any{
						map[string]any{
							"title":    "Go Programming",
							"author":   "John Doe",
							"price":    50,
							"@stock":   "true",
							"language": "en",
						},
						map[string]any{
							"title":    "El Quijote",
							"author":   "Cervantes",
							"price":    30,
							"@stock":   "false",
							"language": "es",
						},
					},
				},
				// Section 1: Science
				map[string]any{
					"@name": "Science",
					"book": map[string]any{ // Single item, not a list (edge case)
						"title":  "Physics 101",
						"author": "Einstein",
					},
				},
			},
			// Complex node with inner #text (not simplified)
			"description": map[string]any{
				"#text": "A place for books",
				"@lang": "en",
			},
		},
	}
}

// ============================================================================
// 1. UNIT TESTS: HELPER FUNCTIONS
// ============================================================================

func TestParseSegment(t *testing.T) {
	tests := []struct {
		input    string
		wantKey  string
		wantFKey string
		wantFVal string
		wantIdx  int
	}{
		{"user", "user", "", "", -1},                      // Simple key
		{"user[0]", "user", "", "", 0},                    // Index 0
		{"user[10]", "user", "", "", 10},                  // Index 10
		{"item[id=5]", "item", "id", "5", -1},             // Filter
		{"item[@type=book]", "item", "@type", "book", -1}, // Attribute Filter
	}

	for _, tt := range tests {
		k, fk, fv, idx := parseSegment(tt.input)
		if k != tt.wantKey || fk != tt.wantFKey || fv != tt.wantFVal || idx != tt.wantIdx {
			t.Errorf("parseSegment(%q) = (%q, %q, %q, %d); want (%q, %q, %q, %d)",
				tt.input, k, fk, fv, idx, tt.wantKey, tt.wantFKey, tt.wantFVal, tt.wantIdx)
		}
	}
}

// ============================================================================
// 2. INTEGRATION TESTS: QUERY ENGINE
// ============================================================================

func TestQuery_BasicNavigation(t *testing.T) {
	data := getQueryTestData()

	tests := []struct {
		path     string
		expected any
	}{
		{"library/@version", "1.0"},
		{"library/info", "City Library"},
		{"library/section[0]/@name", "Fiction"},
		{"library/section[1]/book/author", "Einstein"},
	}

	for _, tt := range tests {
		got, err := Query(data, tt.path)
		if err != nil {
			t.Errorf("Query(%q) unexpected error: %v", tt.path, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("Query(%q) = %v; want %v", tt.path, got, tt.expected)
		}
	}
}

func TestQuery_Indexing(t *testing.T) {
	data := getQueryTestData()

	// 1. Access array index
	title, err := Query(data, "library/section[0]/book[1]/title")
	if err != nil || title != "El Quijote" {
		t.Errorf("Array indexing failed. Got: %v, Want: 'El Quijote'", title)
	}

	// 2. Index out of bounds
	_, err = Query(data, "library/section[0]/book[99]/title")
	if err == nil {
		t.Error("Expected error for out of bounds index, got nil")
	}
}

func TestQuery_Filtering(t *testing.T) {
	data := getQueryTestData()

	// 1. Filter by Child Value
	// section[0] has multiple books. We want the one with language='es'
	title, err := Query(data, "library/section[0]/book[language=es]/title")
	if err != nil {
		t.Fatalf("Filter error: %v", err)
	}
	if title != "El Quijote" {
		t.Errorf("Filter by child value failed. Got %v", title)
	}

	// 2. Filter by Attribute
	// section[0] has multiple books. We want the one with @stock='true'
	title2, err := Query(data, "library/section[0]/book[@stock=true]/title")
	if err != nil {
		t.Fatalf("Filter error: %v", err)
	}
	if title2 != "Go Programming" {
		t.Errorf("Filter by attribute failed. Got %v", title2)
	}
}

func TestQueryAll_DeepSearch(t *testing.T) {
	data := getQueryTestData()

	// We want ALL titles from ALL sections.
	// Path: library -> section (array) -> book (array or single) -> title
	results, err := QueryAll(data, "library/section/book/title")
	if err != nil {
		t.Fatalf("QueryAll error: %v", err)
	}

	// Expected: "Go Programming", "El Quijote", "Physics 101"
	if len(results) != 3 {
		t.Errorf("QueryAll expected 3 titles, got %d: %v", len(results), results)
	}

	// Check content existence (order might vary depending on map iteration if not array)
	// But here 'section' and 'book' (in section 0) are arrays, so order is deterministic.
	expected := []string{"Go Programming", "El Quijote", "Physics 101"}
	for i, v := range results {
		if v != expected[i] {
			t.Errorf("QueryAll index %d: got %v, want %v", i, v, expected[i])
		}
	}
}

func TestQuery_SmartText(t *testing.T) {
	data := getQueryTestData()

	// Case A: The node is ALREADY a string (Parser simplified it)
	// library/info is "City Library" (string)
	// User asks for "library/info/#text"
	val, err := Query(data, "library/info/#text")
	if err != nil {
		t.Errorf("Smart #text failed on primitive: %v", err)
	}
	if val != "City Library" {
		t.Errorf("Smart #text returned %v, want 'City Library'", val)
	}

	// Case B: The node is a Map with explicit #text
	// library/description is map{"#text": "...", "@lang": "en"}
	val2, err := Query(data, "library/description/#text")
	if err != nil {
		t.Errorf("Explicit #text failed: %v", err)
	}
	if val2 != "A place for books" {
		t.Errorf("Explicit #text returned %v", val2)
	}
}

func TestQuery_Errors(t *testing.T) {
	data := getQueryTestData()

	tests := []string{
		"library/invalid",              // Key not found
		"library/section[0]/book[5]",   // Index out of bounds
		"library/section/book[id=999]", // Filter matching nothing
		"",                             // Empty path (returns root as slice, not error, handled in logic check)
	}

	for _, path := range tests {
		if path == "" {
			continue // Skip empty path check here as QueryAll handles it gracefully
		}
		res, err := Query(data, path)
		if err == nil {
			t.Errorf("Expected error for path %q, got result: %v", path, res)
		}
	}
}
