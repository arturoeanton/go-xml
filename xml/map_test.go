package xml

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
)

func TestOrderedMap_Order(t *testing.T) {
	m := NewMap()
	m.Put("Z", 1)
	m.Put("A", 2)
	m.Put("C", 3)

	keys := m.Keys()
	expected := []string{"Z", "A", "C"}

	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("Order mismatch at index %d. Expected %s, got %s", i, expected[i], k)
		}
	}

	// JSON Marshal check
	b, _ := json.Marshal(m)
	jsonStr := string(b)
	expectedJSON := `{"Z":1,"A":2,"C":3}`
	if jsonStr != expectedJSON {
		t.Errorf("JSON order mismatch.\nExpected: %s\nGot:      %s", expectedJSON, jsonStr)
	}
}

func TestOrderedMap_DeepSet(t *testing.T) {
	m := NewMap()
	m.Set("a/b/c", "deep")

	val := m.String("a/b/c")
	if val != "deep" {
		t.Errorf("Deep Set failed. Got %v", val)
	}

	// Verify intermediate nodes are OrderedMaps
	node := m.GetNode("a/b")
	if node == nil {
		t.Error("Intermediate node a/b missing or not OrderedMap")
	}
}

func TestOrderedMap_List(t *testing.T) {
	m := NewMap()
	// Single item list
	m.Set("items/item", "one")

	list := m.List("items/item")
	// Since "one" is a string, List should return empty or ... ?
	// List returns []*OrderedMap. 'one' is string, so it's not included.
	if len(list) != 0 {
		t.Errorf("Expected empty list for string value, got %d", len(list))
	}

	// Create proper list of objects
	m2 := NewMap()
	child1 := NewMap()
	child1.Put("id", 1)
	child2 := NewMap()
	child2.Put("id", 2)

	// We need to manually simulate slice insertion for this test
	// or use a parser to create it.
	// Let's manually construct:
	itemsNode := NewMap()
	itemsNode.Put("item", []any{child1, child2})
	m2.Put("items", itemsNode)

	nodes := m2.List("items/item")
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Int("id") != 1 {
		t.Error("Element 1 mismatch")
	}
}

func TestOrderedMap_MarshalXML(t *testing.T) {
	m := NewMap()
	m.Put("@id", "123")
	m.Put("Child", "Value")

	// Wrap in a root name for standard marshalling context
	type Root struct {
		XMLName xml.Name `xml:"root"`
		Data    *OrderedMap
	}

	// Ideally OrderedMap implements Marshaler, so we can marshal it directly if we provide a StartElement?
	// But encoding/xml usually marshals fields.
	// Let's test if we can marshal the map directly as a member.

	// Actually, custom Marshaler allows: xml.Marshal(m)
	// But xml.Marshal(m) generates <OrderedMap>...</OrderedMap> usually unless valid mapping.
	// Our MarshalXML has signature: (e *xml.Encoder, start xml.StartElement)

	// Let's try direct marshal
	// Note: For *OrderedMap to be marshalled as the root element with a specific name,
	// we usually need a struct wrapper or rely on the caller providing start element.
	// But `xml.Marshal` uses reflect.

	// Test usage via our Encoder first (which is known to work)
	// But the user added `MarshalXML` specifically for `encoding/xml` compatibility.

	// Let's try standard xml.Marshal
	rawXML, err := xml.Marshal(m)
	if err != nil {
		t.Fatalf("Standard xml.Marshal failed: %v", err)
	}

	// With standard Marshal, it uses the type name as tag if not specified?
	// Or it might fail if StartElement isn't passed correctly?
	// The standard lib passes a default start element based on type name or field tag.
	// Output should be roughly: <OrderedMap id="123"><Child>Value</Child></OrderedMap>

	out := string(rawXML)
	if !strings.Contains(out, `id="123"`) {
		t.Errorf("MarshalXML missing attribute. Got: %s", out)
	}
	if !strings.Contains(out, `<Child>Value</Child>`) {
		t.Errorf("MarshalXML missing child. Got: %s", out)
	}
}

func TestOrderedMap_Dump(t *testing.T) {
	m := NewMap()
	m.Put("A", 1)
	m.Put("B", 2)

	dump := m.Dump()
	// Should be JSON indented
	if !strings.Contains(dump, "{\n") {
		t.Error("Dump should be indented")
	}
	if !strings.Contains(dump, `"A": 1`) {
		t.Error("Dump missing content")
	}
}
