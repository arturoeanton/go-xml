package xml

import (
	"bytes"
	"strings"
	"testing"
)

// These tests exercise the Exclusive XML Canonicalization engine directly
// against hand-derived expected output (not official W3C fixture bytes —
// each expectation is worked out from the algorithm's own rules, which is
// what the implementation in c14n.go is built from).

func TestCanonicalizeXML_AttributeSorting(t *testing.T) {
	in := `<root xmlns:b="urn:b" xmlns:a="urn:a" b:x="2" a:y="1" z="3" m="4"></root>`
	got, err := CanonicalizeXML([]byte(in))
	if err != nil {
		t.Fatalf("CanonicalizeXML error: %v", err)
	}

	// Namespace decls sorted by prefix (a before b); unprefixed attrs first
	// sorted by local name (m before z); then namespace-qualified attrs
	// sorted by (namespace URI, local name) — urn:a before urn:b.
	want := `<root xmlns:a="urn:a" xmlns:b="urn:b" m="4" z="3" a:y="1" b:x="2"></root>`
	if string(got) != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

func TestCanonicalizeXML_DefaultNamespace_NotRedeclared(t *testing.T) {
	in := `<root xmlns="urn:default"><child></child></root>`
	got, err := CanonicalizeXML([]byte(in))
	if err != nil {
		t.Fatalf("CanonicalizeXML error: %v", err)
	}

	// child inherits the default namespace unchanged, so it must NOT
	// re-declare xmlns="urn:default" on itself.
	want := `<root xmlns="urn:default"><child></child></root>`
	if string(got) != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
	if strings.Count(string(got), `xmlns="urn:default"`) != 1 {
		t.Errorf("default namespace should be declared exactly once, got: %s", got)
	}
}

func TestCanonicalizeXML_NamespaceRedeclared_WhenIsolated(t *testing.T) {
	// Parse a document where `ds` is declared AND used on the root, and
	// also used (but not re-declared) on a nested child.
	doc := `<ds:root xmlns:ds="urn:dsig"><ds:child></ds:child></ds:root>`
	root, err := parseC14NTree([]byte(doc))
	if err != nil {
		t.Fatalf("parseC14NTree error: %v", err)
	}
	if len(root.children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.children))
	}
	child := root.children[0]
	if child.local != "child" || child.prefix != "ds" {
		t.Fatalf("unexpected child node: %+v", child)
	}

	// Canonicalizing the WHOLE tree: ds is declared once, on root (the
	// first point where it's visibly utilized), not repeated on the child.
	var wholeBuf bytes.Buffer
	renderCanonicalNode(&wholeBuf, root, map[string]string{}, &c14nConfig{})
	wantWhole := `<ds:root xmlns:ds="urn:dsig"><ds:child></ds:child></ds:root>`
	if wholeBuf.String() != wantWhole {
		t.Errorf("whole-tree: got %s, want %s", wholeBuf.String(), wantWhole)
	}

	// Canonicalizing the SAME child node object in ISOLATION (as its own
	// canonicalization root, e.g. computing a detached signature over just
	// this subtree): ds must be freshly (re)declared on the child itself,
	// even though in the source document it was only declared on root. This
	// is the Exclusive-C14N property that makes "sign this subtree now,
	// embed it anywhere later" safe.
	var isolatedBuf bytes.Buffer
	renderCanonicalNode(&isolatedBuf, child, map[string]string{}, &c14nConfig{})
	wantIsolated := `<ds:child xmlns:ds="urn:dsig"></ds:child>`
	if isolatedBuf.String() != wantIsolated {
		t.Errorf("isolated: got %s, want %s", isolatedBuf.String(), wantIsolated)
	}
}

func TestCanonicalizeXML_EmptyElementsExpanded(t *testing.T) {
	in := `<root><a/><b></b></root>`
	got, err := CanonicalizeXML([]byte(in))
	if err != nil {
		t.Fatalf("CanonicalizeXML error: %v", err)
	}
	want := `<root><a></a><b></b></root>`
	if string(got) != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

func TestCanonicalizeXML_TextEscaping(t *testing.T) {
	// '<' can't appear literally in text; use entities/char-refs in the
	// source the way any XML producer would, then verify our canonicalizer
	// re-escapes them consistently on output.
	in := `<root>x &lt; y &gt; z &amp; w &#xD;end</root>`
	got, err := CanonicalizeXML([]byte(in))
	if err != nil {
		t.Fatalf("CanonicalizeXML error: %v", err)
	}
	want := `<root>x &lt; y &gt; z &amp; w &#xD;end</root>`
	if string(got) != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

func TestCanonicalizeNode_AttributeEscaping(t *testing.T) {
	// Attribute-value whitespace (tab/newline/CR) is normalized away by any
	// compliant XML parser before we ever see it, so to test that our
	// renderer re-escapes those characters we construct the node tree
	// directly instead of round-tripping through XML text.
	node := &c14nNode{
		kind:  c14nElem,
		local: "root",
		attrs: []c14nAttrNode{
			{local: "v", value: "a<b>c&d\"e\tf\ng\rh"},
		},
	}
	var buf bytes.Buffer
	renderCanonicalNode(&buf, node, map[string]string{}, &c14nConfig{})

	want := `<root v="a&lt;b&gt;c&amp;d&quot;e&#x9;f&#xA;g&#xD;h"></root>`
	if buf.String() != want {
		t.Errorf("got:  %s\nwant: %s", buf.String(), want)
	}
}

func TestCanonicalizeXML_Comments(t *testing.T) {
	in := `<root><!-- hello --><a></a></root>`

	withoutComments, err := CanonicalizeXML([]byte(in))
	if err != nil {
		t.Fatalf("CanonicalizeXML error: %v", err)
	}
	want := `<root><a></a></root>`
	if string(withoutComments) != want {
		t.Errorf("default (no comments): got %s, want %s", withoutComments, want)
	}

	withComments, err := CanonicalizeXML([]byte(in), WithComments())
	if err != nil {
		t.Fatalf("CanonicalizeXML error: %v", err)
	}
	wantWith := `<root><!-- hello --><a></a></root>`
	if string(withComments) != wantWith {
		t.Errorf("WithComments(): got %s, want %s", withComments, wantWith)
	}
}

func TestCanonicalizeXML_XMLPrefix_NeverDeclared(t *testing.T) {
	in := `<root xml:lang="es"></root>`
	got, err := CanonicalizeXML([]byte(in))
	if err != nil {
		t.Fatalf("CanonicalizeXML error: %v", err)
	}
	want := `<root xml:lang="es"></root>`
	if string(got) != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
	if strings.Contains(string(got), "xmlns:xml") {
		t.Errorf("must never emit an explicit xmlns:xml declaration, got: %s", got)
	}
}

func TestCanonicalizeXML_Idempotent(t *testing.T) {
	in := `<root xmlns:ds="urn:dsig" b="2" a="1"><ds:child>text &amp; more</ds:child></root>`
	first, err := CanonicalizeXML([]byte(in))
	if err != nil {
		t.Fatalf("first CanonicalizeXML error: %v", err)
	}
	second, err := CanonicalizeXML(first)
	if err != nil {
		t.Fatalf("second CanonicalizeXML error: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("canonicalization is not idempotent.\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestCanonicalize_OrderedMap(t *testing.T) {
	inner := NewMap()
	inner.Set("@z", "last")
	inner.Set("@a", "first")
	inner.Set("Child", "value")

	root := NewMap()
	root.Put("Root", inner)

	got, err := Canonicalize(root)
	if err != nil {
		t.Fatalf("Canonicalize error: %v", err)
	}
	want := `<Root a="first" z="last"><Child>value</Child></Root>`
	if string(got) != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

func TestCanonicalize_ErrorsWithoutRootKey(t *testing.T) {
	// Canonicalize now requires a document-shaped map (single root key),
	// the same convention Marshal/MapXML already enforce elsewhere. A map
	// with only attributes and no element key can't be marshaled to XML at
	// all, so this must return an error rather than silently emitting
	// malformed output the way the pre-rewrite implementation did.
	m := NewMap()
	m.Set("@a", "1")

	if _, err := Canonicalize(m); err == nil {
		t.Fatal("expected error for a map with no root element key, got nil")
	}
}
