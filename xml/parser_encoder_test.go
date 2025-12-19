package xml

import (
	"strings"
	"testing"
)

func TestParserEncoder_Roundtrip(t *testing.T) {
	// Complex XML with attributes, nested elements, and mixed content support
	inputXML := `<Root id="123" type="test"><ChildA>ValueA</ChildA><ChildB attr="b">ValueB</ChildB></Root>`

	// 1. Parse
	om, err := MapXML(strings.NewReader(inputXML))
	if err != nil {
		t.Fatalf("MapXML failed: %v", err)
	}

	// Verify Structure
	// MapXML returns a wrapper map containing the root element key.
	if !om.Has("Root") {
		t.Fatalf("Parsed map does not contain Root key. Keys: %v", om.Keys())
	}

	// Navigate to inner content
	rootNode := om.GetNode("Root")
	if rootNode == nil {
		t.Fatal("Root node is nil")
	}

	if rootNode.String("@id") != "123" {
		t.Errorf("Inner @id mismatch. Got %v", rootNode.Get("@id"))
	}

	// 2. Encode
	outXML, err := Marshal(om)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// 3. Compare
	if outXML != inputXML {
		t.Errorf("Roundtrip mismatch.\nInput:  %s\nOutput: %s", inputXML, outXML)
	}
}

func TestParserEncoder_Attributes(t *testing.T) {
	// Test attribute priority in encoding
	m := NewMap()

	// Build <Tag id="1"><Child></Child></Tag>
	rootContent := NewMap()
	rootContent.Put("@id", "1")
	rootContent.Put("Child", "")

	m.Put("Tag", rootContent)

	out, err := Marshal(m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `<Tag id="1"><Child></Child></Tag>`
	if out != expected {
		t.Errorf("Attribute placement error.\nExpected: %s\nGot:      %s", expected, out)
	}
}
