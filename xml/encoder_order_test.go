package xml

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncoder_OrderedMap_PreservesOrder(t *testing.T) {
	// 1. Create OrderedMap with specific insertion order
	// Order: Z, A, C (Anti-alphabetical/Random)
	root := NewMap()
	root.Put("Zebra", "Animal")
	root.Put("Apple", "Fruit")
	root.Put("Carrot", "Vegetable")

	// Wrap in a root element because Encoder requires single root
	doc := NewMap()
	doc.Put("Root", root)

	// 2. Encode
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	err := enc.Encode(doc)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	xmlStr := buf.String()

	// 3. Verify Order
	// We expect <Root><Zebra>...</Zebra><Apple>...</Apple><Carrot>...</Carrot></Root>
	// Standard map would likely sort this as Apple, Carrot, Zebra.

	expectedOrder := []string{"<Zebra>", "<Apple>", "<Carrot>"}
	lastIndex := -1

	for _, tag := range expectedOrder {
		idx := strings.Index(xmlStr, tag)
		if idx == -1 {
			t.Errorf("Tag %s not found in output: %s", tag, xmlStr)
		}
		if idx < lastIndex {
			t.Errorf("Tag %s appeared out of order. Expected %v. Output: %s", tag, expectedOrder, xmlStr)
		}
		lastIndex = idx
	}
}

func TestEncoder_LegacyMap_SortsKeys(t *testing.T) {
	// 1. Create standard map (random iteration order)
	// Keys: C, A, B
	data := map[string]any{
		"Root": map[string]any{
			"Carrot": "Vegetable",
			"Apple":  "Fruit",
			"Banana": "Fruit",
		},
	}

	// 2. Encode
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	xmlStr := buf.String()

	// 3. Verify Order (Should be Alphabetical: Apple, Banana, Carrot)
	expectedOrder := []string{"<Apple>", "<Banana>", "<Carrot>"}
	lastIndex := -1

	for _, tag := range expectedOrder {
		idx := strings.Index(xmlStr, tag)
		if idx == -1 {
			t.Errorf("Tag %s not found in output: %s", tag, xmlStr)
		}
		if idx < lastIndex {
			t.Errorf("Tag %s appeared out of order (Should be sorted). Output: %s", tag, xmlStr)
		}
		lastIndex = idx
	}
}

func TestEncoder_Marshal_OrderedMap(t *testing.T) {
	root := NewMap()
	root.Put("@id", "1")
	root.Put("Name", "Test")

	doc := NewMap()
	doc.Put("Item", root)

	s, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `<Item id="1"><Name>Test</Name></Item>`
	if s != expected {
		t.Errorf("Marshal(OrderedMap) mismatch.\nGot:  %s\nWant: %s", s, expected)
	}
}
