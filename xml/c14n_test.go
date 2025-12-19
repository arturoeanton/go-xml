package xml

import (
	"testing"
)

func TestCanonicalize_Attributes_Sorted(t *testing.T) {
	// Setup: Map with unsorted attributes
	m := NewMap()
	m.Set("@z", "last")
	m.Set("@a", "first")
	m.Set("@m", "middle")
	m.Set("child", "value")

	// Call
	b, err := Canonicalize(m)
	if err != nil {
		t.Fatalf("Canonicalize failed: %v", err)
	}

	// Verify: Attributes should be strictly ordered a="first" m="middle" z="last"
	// Note: The function wraps this in a logic that might depend on how it's called (writeMapCanonical vs Canonicalize)
	// Canonicalize calls writeMapCanonical with tagName="" -> so it just writes attributes then children?
	// Let's check c14n.go...
	// If tagName is "", it doesn't print <tagName ...>.
	// It prints attributes?
	// Wait, line 55 in c14n.go: if tagName != "" { buf.WriteByte('<'); ... }
	// So if we pass a map directly to Canonicalize, it effectively treats it as root content?
	// ACTUALLY, line 20: writeCanonical(..., tagName="").
	// Line 23 -> writeMapCanonical(..., tagName="")
	// Line 55 -> if tagName != "" checks are skipped.
	// Attributes are written: buf.WriteString(attrName) ...
	// Wait, if tagName is empty, where do attributes go?
	// Line 72 loop writes attributes: ' ' + attrName + '="' + val + '"'
	// So if tagName is empty, it starts with a space? " a="first" m="middle" z="last"child" ??
	// Let's re-read c14n.go.

	// If tagName is empty:
	// 1. Tag open skipped.
	// 2. Attributes written: " a="first"..."
	// 3. Tag close skipped.
	// 4. Content written.
	// 5. Closing tag skipped.

	// This seems like Canonicalize expects a "Root" wrapper or similar if we want a valid XML tag.
	// BUT, for testing just the canonicalization rules (sorting), this output is sufficient to check order.

	got := string(b)
	// Expected: " a=\"first\" m=\"middle\" z=\"last\"<child>value</child>"
	// or similar depending on child handling using NewMap (child order preserved?)
	// Actually NewMap preserves insertion order for non-attributes.
	// But attributes are sorted.

	// Let's check expected string more loosely or construct exact expectation
	expectedSnippet := ` a="first" m="middle" z="last"`
	if !containsStr(got, expectedSnippet) {
		t.Errorf("Attributes not sorted/present. Got: %s, Want snippet: %s", got, expectedSnippet)
	}
}

func TestCanonicalize_FullStructure(t *testing.T) {
	// Real use case usually has a root element.
	root := NewMap()
	root.Set("@id", "123")
	child := NewMap()
	child.Set("#text", "foo")
	root.Set("child", child)

	// Since Canonicalize takes `v interface{}`, if we pass the map, it has empty tagName.
	// To get <root id="123"><child>foo</child></root>, we need to pass strict structure?
	// c14n.go seems to imply we pass the inner content of a tag?
	// OR we are supposed to wrap it?

	// Looking at singer.go usage:
	// xpWrapper := NewMap()
	// xpWrapper.Set("@xmlns:xades", ...)
	// xpWrapper.Set("xades:SignedProperties", ...)
	// xpBytes, err := Canonicalize(xpWrapper)

	// So if xpWrapper is the "root" object passed to Canonicalize:
	// writeCanonical(..., tagName="")
	//   writeMapCanonical(..., tagName="")
	//     Attributes of xpWrapper are written with leading space.
	//     Children of xpWrapper are written.
	// So output is: ` xmlns:xades="..."<xades:SignedProperties>...</xades:SignedProperties>`
	// This generates a "fragment" of attributes + content, but NOT the surrounding <Root> tag?
	// Ah, C14N usually canonicalizes a node. If we pass the map representing the node attributes+children,
	// it generates the inside?
	// Wait, standard C14N usually includes the node itself.
	// Is c14n.go assuming the caller prints the start/end tags of the *root*?
	// Helper.go Marshal usually handles the root?
	// Let's see how c14n.go handles recursion.
	// Recurse: writeCanonical(buf, val, k) -> k is the tagName.
	// So children GET their tags.
	// The Top-level map passed to Canonicalize gets tagName="" (line 14).
	// So the top level map is treated as "Attributes and Content of the context node", but NOT the context node tag itself?

	// Let's verifying this behavior with this test.
	gotBytes, err := Canonicalize(root)
	if err != nil {
		t.Fatalf("Canonicalize error: %v", err)
	}
	got := string(gotBytes)

	// Expectation:
	// Attributes:  id="123" (sorted)
	// Content: <child>foo</child>
	// Total: ` id="123"<child>foo</child>`
	expected := ` id="123"<child>foo</child>`
	if got != expected {
		t.Errorf("Got %q, want %q", got, expected)
	}
}

func TestCanonicalize_Nested_Elements(t *testing.T) {
	grandchild := NewMap()
	grandchild.Set("@name", "gran")
	grandchild.Set("#text", "GC") // <grandchild name="gran">GC</grandchild>

	child := NewMap()
	child.Set("grandchild", grandchild)

	// To test "child" having a tag, we need to correct how we construct the input or expectations
	// If I put "child" in a map, it becomes a key.
	rootMap := NewMap()
	rootMap.Set("child", child)

	b, _ := Canonicalize(rootMap)
	got := string(b)
	// rootMap has no attrs, so no leading space/attrs.
	// Has "child" key -> recurses with tagName="child"
	// writeMapCanonical(..., tagName="child")
	//   -> <child>
	//   -> attrs of child (none)
	//   -> content: "grandchild": ...
	//      -> <grandchild> ...
	expected := `<child><grandchild name="gran">GC</grandchild></child>`

	if got != expected {
		t.Errorf("Nesting failed.\nGot:  %s\nWant: %s", got, expected)
	}
}

func TestCanonicalize_Text_Escaping(t *testing.T) {
	m := NewMap()
	// Test chars: < > & " (newlines handled in attrs)
	// Attribute val: " < &
	m.Set("@attr", "a < b & c \" d")
	// Text val: < > &
	m.Set("#text", "x < y > z & w")

	b, _ := Canonicalize(m)
	got := string(b)

	// Expected Attr: "a &lt; b &amp; c &quot; d"
	// Expected Text: "x &lt; y &gt; z &amp; w"
	// Output format: ` attr="..."x ...`

	expectedAttr := ` attr="a &lt; b &amp; c &quot; d"`
	expectedText := `x &lt; y &gt; z &amp; w`
	expected := expectedAttr + expectedText

	if got != expected {
		t.Errorf("Escaping failed.\nGot:  %s\nWant: %s", got, expected)
	}
}

func TestCanonicalize_Arrays(t *testing.T) {
	// <list><item>1</item><item>2</item></list>
	// Modeled as:
	// root has key "list" -> map
	// map has key "item" -> []interface{}{"1", "2"} OR []Map

	item1 := NewMap()
	item1.Set("#text", "1")
	item2 := NewMap()
	item2.Set("#text", "2")

	list := NewMap()
	list.Set("item", []interface{}{item1, item2})

	root := NewMap()
	root.Set("list", list)

	b, _ := Canonicalize(root)
	got := string(b)

	expected := `<list><item>1</item><item>2</item></list>`
	if got != expected {
		t.Errorf("Array handling failed.\nGot:  %s\nWant: %s", got, expected)
	}
}

func TestCanonicalize_Mixed_Types(t *testing.T) {
	m := NewMap()
	m.Set("int", 100)
	m.Set("float", 3.14)
	m.Set("bool", true)

	b, _ := Canonicalize(m)
	got := string(b)

	// Map iteration order is preserved in OrderedMap
	// keys: int, float, bool (insertion order)
	// But wait, if I Inserted them in that order.
	// int->100
	// float->3.14
	// bool->true

	// <int>100</int><float>3.14</float><bool>true</bool>
	expected := `<int>100</int><float>3.14</float><bool>true</bool>`
	if got != expected {
		t.Errorf("Mixed types failed.\nGot:  %s\nWant: %s", got, expected)
	}
}

func containsStr(s, substr string) bool {
	// Simple helper if needed, but strings.Contains is standard
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
