package xml

import (
	"testing"
)

func TestHelper_HybridQuery(t *testing.T) {
	// 1. Native Map
	native := map[string]any{
		"A": map[string]any{
			"B": "Value",
		},
	}

	// 2. OrderedMap
	ordered := NewMap()
	child := NewMap()
	child.Put("B", "Value")
	ordered.Put("A", child)

	// Test Logic
	check := func(name string, data any) {
		val, err := Query(data, "A/B")
		if err != nil {
			t.Errorf("[%s] Query failed: %v", name, err)
			return
		}
		if val != "Value" {
			t.Errorf("[%s] Expected 'Value', got %v", name, val)
		}

		valT, errT := Get[string](data, "A/B")
		if errT != nil {
			t.Errorf("[%s] Get failed: %v", name, errT)
		}
		if valT != "Value" {
			t.Errorf("[%s] Get expected 'Value', got %v", name, valT)
		}
	}

	check("Native", native)
	check("Ordered", ordered)
}

func TestHelper_MixedNesting(t *testing.T) {
	// Ordered -> Native -> Ordered
	root := NewMap()
	nativeChild := map[string]any{
		"deep": NewMap(),
	}
	// Inject ordered map into native map (valid as 'any')
	nativeChild["deep"].(*OrderedMap).Put("final", 999)

	root.Put("mid", nativeChild)

	// Path: mid / deep / final
	// mid is map[string]any
	// deep is *OrderedMap
	// final is 999 (int)

	val, err := Get[int](root, "mid/deep/final")
	if err != nil {
		t.Fatalf("Mixed query failed: %v", err)
	}
	if val != 999 {
		t.Errorf("Expected 999, got %v", val)
	}
}
