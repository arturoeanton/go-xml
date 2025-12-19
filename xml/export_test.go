package xml

import (
	"bytes"
	"strings"
	"testing"
)

func TestToCSV_Basic(t *testing.T) {
	// 1. Data Setup
	// Item 1: Complete
	item1 := NewMap()
	item1.Put("id", "101")
	item1.Put("name", "Widget")
	item1.Put("cost", "10.50")

	// Item 2: Missing 'cost', extra 'color'
	item2 := NewMap()
	item2.Put("id", "102")
	item2.Put("name", "Gadget")
	item2.Put("color", "blue")

	list := []*OrderedMap{item1, item2}

	// 2. Execute
	var buf bytes.Buffer
	if err := ToCSV(&buf, list); err != nil {
		t.Fatalf("ToCSV failed: %v", err)
	}

	output := buf.String()

	// 3. Verify
	// Headers should be sorted: color, cost, id, name
	expectedHeader := "color,cost,id,name"
	if !strings.Contains(output, expectedHeader) {
		t.Errorf("Header mismatch.\nExpected: %s\nGot: %s", expectedHeader, output)
	}

	// Verify Item 1: color="", cost="10.50", id="101", name="Widget"
	// Note: Strings in CSV might vary slightly depending on logic (empty vs quoted empty), but current implementation uses simple empty string.
	// Expected line: ,10.50,101,Widget
	if !strings.Contains(output, ",10.50,101,Widget") {
		t.Errorf("Item 1 data mismatch/missing.\nGot:\n%s", output)
	}

	// Verify Item 2: color="blue", cost="", id="102", name="Gadget"
	// Expected line: blue,,102,Gadget
	if !strings.Contains(output, "blue,,102,Gadget") {
		t.Errorf("Item 2 data mismatch/missing.\nGot:\n%s", output)
	}
}

func TestToCSV_Escaping(t *testing.T) {
	node := NewMap()
	node.Put("desc", `Large "Red" Apple, Fuji`) // Contains quotes and comma
	node.Put("id", "55")

	list := []*OrderedMap{node}

	var buf bytes.Buffer
	if err := ToCSV(&buf, list); err != nil {
		t.Fatalf("ToCSV failed: %v", err)
	}

	output := buf.String()

	// Expect: desc,id
	// Expect val: "Large ""Red"" Apple, Fuji",55
	expectedFragment := `"Large ""Red"" Apple, Fuji",55`

	if !strings.Contains(output, expectedFragment) {
		t.Errorf("Escaping failed.\nExpected fragment: %s\nFull Output:\n%s", expectedFragment, output)
	}
}

func TestToCSV_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := ToCSV(&buf, []*OrderedMap{}); err != nil {
		t.Fatalf("ToCSV failed on empty: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("Expected empty output, got %d bytes", buf.Len())
	}
}

func TestReaderToJSON(t *testing.T) {
	xmlData := `<root><item>A</item><item>B</item></root>`
	r := strings.NewReader(xmlData)

	jsonBytes, err := ReaderToJSON(r)
	if err != nil {
		t.Fatalf("ReaderToJSON failed: %v", err)
	}

	str := string(jsonBytes)
	// Expected JSON: {"item":["A","B"]} (roughly, or nested depending on MapXML logic)
	// Actually MapXML returns root wrapper: {"root":{"item":["A","B"]}}

	if !strings.Contains(str, `"root"`) {
		t.Errorf("JSON missing root key: %s", str)
	}
	if !strings.Contains(str, `"item"`) {
		t.Errorf("JSON missing item key: %s", str)
	}
}
