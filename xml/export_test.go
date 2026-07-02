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

func TestToJSON_OrderedMap(t *testing.T) {
	m := NewMap()
	m.Put("b", 2)
	m.Put("a", 1)

	got, err := ToJSON(m)
	if err != nil {
		t.Fatalf("ToJSON(*OrderedMap) error: %v", err)
	}
	want := `{"b":2,"a":1}`
	if got != want {
		t.Errorf("ToJSON(*OrderedMap) = %s, want %s", got, want)
	}
}

func TestToJSON_Reader(t *testing.T) {
	got, err := ToJSON(strings.NewReader(`<root><a>1</a></root>`))
	if err != nil {
		t.Fatalf("ToJSON(io.Reader) error: %v", err)
	}
	if !strings.Contains(got, `"a"`) {
		t.Errorf("ToJSON(io.Reader) missing expected content: %s", got)
	}
}

func TestToJSON_Fallback(t *testing.T) {
	got, err := ToJSON(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("ToJSON(fallback) error: %v", err)
	}
	if got != `{"x":1}` {
		t.Errorf("ToJSON(fallback) = %s, want {\"x\":1}", got)
	}
}

func TestToCSVWithOptions_Delimiter(t *testing.T) {
	item := NewMap()
	item.Put("id", "1")
	item.Put("name", "Widget")

	var buf bytes.Buffer
	if err := ToCSVWithOptions(&buf, []*OrderedMap{item}, WithDelimiter(';')); err != nil {
		t.Fatalf("ToCSVWithOptions error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "id;name") {
		t.Errorf("expected ';'-delimited header, got: %s", got)
	}
	if !strings.Contains(got, "1;Widget") {
		t.Errorf("expected ';'-delimited row, got: %s", got)
	}
}

func TestToCSVWithOptions_QuoteAll(t *testing.T) {
	item := NewMap()
	item.Put("id", "1")
	item.Put("name", "Widget")

	var buf bytes.Buffer
	if err := ToCSVWithOptions(&buf, []*OrderedMap{item}, WithQuoteAll(true)); err != nil {
		t.Fatalf("ToCSVWithOptions error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"id","name"`) {
		t.Errorf("expected fully-quoted header, got: %s", got)
	}
	if !strings.Contains(got, `"1","Widget"`) {
		t.Errorf("expected fully-quoted row, got: %s", got)
	}
}

func TestToCSVWithOptions_Flatten(t *testing.T) {
	addr := NewMap()
	addr.Put("city", "Bogota")
	addr.Put("zip", "110111")

	item := NewMap()
	item.Put("id", "1")
	item.Put("address", addr)

	var buf bytes.Buffer
	if err := ToCSVWithOptions(&buf, []*OrderedMap{item}, WithFlatten(".")); err != nil {
		t.Fatalf("ToCSVWithOptions error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "address.city") || !strings.Contains(got, "address.zip") {
		t.Errorf("expected flattened headers address.city/address.zip, got: %s", got)
	}
	if !strings.Contains(got, "Bogota") || !strings.Contains(got, "110111") {
		t.Errorf("expected flattened values, got: %s", got)
	}
}

func TestToCSVWithOptions_DefaultSkipsNested(t *testing.T) {
	addr := NewMap()
	addr.Put("city", "Bogota")

	item := NewMap()
	item.Put("id", "1")
	item.Put("address", addr)

	var buf bytes.Buffer
	if err := ToCSVWithOptions(&buf, []*OrderedMap{item}); err != nil {
		t.Fatalf("ToCSVWithOptions error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "city") {
		t.Errorf("without WithFlatten, nested objects should be skipped, got: %s", got)
	}
}
