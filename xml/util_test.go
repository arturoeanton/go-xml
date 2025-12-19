package xml

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// ============================================================================
// 1. TYPE COERCION TESTS
// ============================================================================

func TestAsTypeWrappers(t *testing.T) {
	// 1. AsString
	t.Run("AsString", func(t *testing.T) {
		tests := []struct {
			input    any
			expected string
		}{
			{"hello", "hello"},
			{123, "123"},
			{true, "true"},
			{nil, ""},
			{[]byte("bytes"), "bytes"},
			{map[string]int{"a": 1}, `{"a":1}`}, // JSON fallback
		}

		for _, tt := range tests {
			if got := AsString(tt.input); got != tt.expected {
				t.Errorf("AsString(%v) = %q; want %q", tt.input, got, tt.expected)
			}
		}
	})

	// 2. AsInt
	t.Run("AsInt", func(t *testing.T) {
		tests := []struct {
			input    any
			expected int
		}{
			{10, 10},
			{10.9, 10},   // Truncate
			{" 42 ", 42}, // Trim & Parse
			{true, 1},
			{false, 0},
			{"invalid", 0},
			{nil, 0},
		}

		for _, tt := range tests {
			if got := AsInt(tt.input); got != tt.expected {
				t.Errorf("AsInt(%v) = %d; want %d", tt.input, got, tt.expected)
			}
		}
	})

	// 3. AsBool
	t.Run("AsBool", func(t *testing.T) {
		tests := []struct {
			input    any
			expected bool
		}{
			{true, true},
			{"true", true},
			{"TRUE", true},
			{"1", true},
			{"yes", true},
			{"on", true},
			{"false", false},
			{0, false},
			{nil, false},
			{"random", false},
		}

		for _, tt := range tests {
			if got := AsBool(tt.input); got != tt.expected {
				t.Errorf("AsBool(%v) = %v; want %v", tt.input, got, tt.expected)
			}
		}
	})

	// 4. AsTime
	t.Run("AsTime", func(t *testing.T) {
		// Valid case
		t1, err := AsTime("2023-01-01", "2006-01-02")
		if err != nil || t1.Year() != 2023 {
			t.Errorf("AsTime valid failed: %v", err)
		}

		// Invalid case
		_, err = AsTime("invalid-date")
		if err == nil {
			t.Error("AsTime should have failed for invalid date")
		}
	})

	// 5. AsSlice
	t.Run("AsSlice", func(t *testing.T) {
		// Nil -> Empty slice
		if res := AsSlice(nil); len(res) != 0 {
			t.Error("AsSlice(nil) should be empty")
		}
		// Single item -> Slice of 1
		if res := AsSlice("item"); len(res) != 1 || res[0] != "item" {
			t.Error("AsSlice(single) failed")
		}
		// Slice -> Slice
		input := []any{1, 2}
		if res := AsSlice(input); len(res) != 2 {
			t.Error("AsSlice(slice) failed")
		}
	})
}

// ============================================================================
// 2. MAP INSPECTION TESTS
// ============================================================================

func TestMapHelpers(t *testing.T) {
	data := map[string]any{
		"name":   "Gopher",
		"@id":    "123",
		"#text":  "content",
		"nested": 1,
	}

	// Keys
	t.Run("Keys", func(t *testing.T) {
		keys := Keys(data)
		// Map iteration is random, but Keys() must sort them
		expected := []string{"#text", "@id", "name", "nested"}
		if !reflect.DeepEqual(keys, expected) {
			t.Errorf("Keys() = %v; want %v", keys, expected)
		}
	})

	// Attributes
	t.Run("Attributes", func(t *testing.T) {
		attrs := Attributes(data)
		if len(attrs) != 1 || attrs["@id"] != "123" {
			t.Errorf("Attributes() failed: %v", attrs)
		}
	})

	// Children
	t.Run("Children", func(t *testing.T) {
		children := Children(data)
		if len(children) != 2 || children["name"] != "Gopher" {
			t.Errorf("Children() failed: %v", children)
		}
		if _, hasAttr := children["@id"]; hasAttr {
			t.Error("Children() should not contain attributes")
		}
	})

	// Pick & Omit
	t.Run("PickOmit", func(t *testing.T) {
		picked := Pick(data, "name", "@id")
		if len(picked) != 2 {
			t.Error("Pick failed")
		}

		omitted := Omit(data, "name", "@id")
		if _, exists := omitted["name"]; exists {
			t.Error("Omit failed (name still exists)")
		}
	})
}

// ============================================================================
// 3. STRUCTURAL TRANSFORMATION TESTS
// ============================================================================

// ============================================================================
// 3. STRUCTURAL TRANSFORMATION TESTS
// ============================================================================

func TestTransformations(t *testing.T) {
	// Merge
	t.Run("Merge", func(t *testing.T) {
		base := map[string]any{
			"a": 1,
			"sub": map[string]any{
				"x": 10,
			},
		}
		override := map[string]any{
			"b": 2,
			"sub": map[string]any{
				"y": 20, // Should merge into sub, not replace
			},
		}

		Merge(base, override)

		// Nota: Aquí usamos AsInt para asegurar la comparación si hubiera conversiones,
		// pero Merge mantiene los tipos originales.
		if AsInt(base["a"]) != 1 || AsInt(base["b"]) != 2 {
			t.Error("Merge: Top level failed")
		}
		sub := base["sub"].(map[string]any)
		if AsInt(sub["x"]) != 10 || AsInt(sub["y"]) != 20 {
			t.Error("Merge: Recursive merge failed")
		}
	})

	// Clone
	t.Run("Clone", func(t *testing.T) {
		original := map[string]any{"a": 1}
		copyVal := Clone(original)
		copyMap := copyVal.(map[string]any)

		copyMap["a"] = 2
		// Verificamos que el original siga siendo 1
		if AsInt(original["a"]) != 1 {
			t.Error("Clone failed: modifying copy affected original")
		}
	})

	// Flatten
	t.Run("Flatten", func(t *testing.T) {
		nested := map[string]any{
			"a": map[string]any{
				"b": map[string]any{
					"c": 1, // Esto es un int
				},
			},
			"d": 2, // Esto es un int
		}
		flat := Flatten(nested)

		// CORRECCIÓN: Flatten preserva tipos, así que esperamos int (1), no float64 (1.0).
		// Usamos aserción directa de valor.
		if flat["a.b.c"] != 1 {
			t.Errorf("Flatten failed on a.b.c: got %T %v, expected int 1", flat["a.b.c"], flat["a.b.c"])
		}
		if flat["d"] != 2 {
			t.Errorf("Flatten failed on d: got %T %v, expected int 2", flat["d"], flat["d"])
		}
	})

	// Text (Deep Extraction)
	t.Run("Text", func(t *testing.T) {
		// Simulating HTML: <div><p>Hello <b>World</b></p></div>
		data := map[string]any{
			"div": map[string]any{
				"p": map[string]any{
					"#text": "Hello",
					"b": map[string]any{
						"#text": "World",
					},
				},
			},
		}
		txt := Text(data)
		// Como el orden de iteración de mapas es aleatorio en Go, verificamos contenido.
		if !strings.Contains(txt, "Hello") || !strings.Contains(txt, "World") {
			t.Errorf("Text extraction missing content: %s", txt)
		}
	})
}

// ============================================================================
// 4. FUNCTIONAL HELPER TESTS
// ============================================================================

func TestFunctional(t *testing.T) {
	nums := []int{1, 2, 3, 4}

	// MapSlice
	strs := MapSlice(nums, func(i int) string {
		return strconv.Itoa(i * 10)
	})
	if strs[0] != "10" || strs[3] != "40" {
		t.Error("MapSlice failed")
	}

	// FilterSlice
	evens := FilterSlice(nums, func(i int) bool {
		return i%2 == 0
	})
	if len(evens) != 2 || evens[0] != 2 {
		t.Error("FilterSlice failed")
	}

	// FindFirst
	found, ok := FindFirst(nums, func(i int) bool {
		return i == 3
	})
	if !ok || found != 3 {
		t.Error("FindFirst failed")
	}
}

// ============================================================================
// 5. SERIALIZATION TESTS
// ============================================================================

func TestBinding(t *testing.T) {
	data := map[string]any{
		"name": "Alice",
		"age":  30,
	}

	// MapToJSON
	jsonStr, _ := MapToJSON(data)
	if len(jsonStr) == 0 {
		t.Error("MapToJSON returned empty")
	}

	// MapToStruct
	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	var u User
	if err := MapToStruct(data, &u); err != nil {
		t.Fatalf("MapToStruct failed: %v", err)
	}

	if u.Name != "Alice" || u.Age != 30 {
		t.Errorf("MapToStruct mapping error: %+v", u)
	}
}

func TestToJSON_Reader(t *testing.T) {
	xmlData := `<user id="1"><name>Alice</name></user>`
	r := strings.NewReader(xmlData)

	jsonBytes, err := ToJSON(r)
	if err != nil {
		t.Fatalf("ToJSON(Reader) failed: %v", err)
	}

	jsonStr := string(jsonBytes)
	// Simple check: ensure it contains keys. JSON order is random unless sorted or small.
	if !strings.Contains(jsonStr, "user") || !strings.Contains(jsonStr, "Alice") {
		t.Errorf("ToJSON(Reader) output unexpected: %s", jsonStr)
	}

	if jsonStr != `{"user":{"id":"1","name":"Alice"}}` {
		t.Errorf("ToJSON(Reader) output unexpected: %s", jsonStr)
	}

}
