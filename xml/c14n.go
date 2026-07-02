package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ============================================================================
// EXCLUSIVE XML CANONICALIZATION (http://www.w3.org/2001/10/xml-exc-c14n#)
//
// This is the canonicalization algorithm actually used in practice for
// enveloped XML-DSig signatures, WS-Security and XAdES-BES (including
// Colombian DIAN e-invoicing): unlike Inclusive C14N, the canonical form of
// a subtree only depends on the namespaces it visibly uses, never on where
// it ends up embedded later. That property is what lets Signer.CreateSignature
// / CreateXadesSignature compute a signature over a detached SignedInfo node
// and have it still validate once embedded anywhere in the final document.
//
// Known, deliberate limitations (documented rather than silently wrong):
//   - No "InclusiveNamespaces PrefixList" support (the escape hatch for
//     namespace prefixes referenced only inside attribute VALUES / QName
//     content, which the algorithm can't detect automatically).
//   - No XML attribute inheritance (xml:lang/xml:space copy-down) — that is
//     an Inclusive C14N 1.0 rule, not part of Exclusive C14N; implementing it
//     here would produce non-interoperable output.
//   - Namespace prefixes are taken verbatim from the source document (via
//     encoding/xml's RawToken, which preserves literal prefixes instead of
//     resolving them away) rather than reassigned, which is required for the
//     digest computed here to match what an external validator recomputes
//     from the same embedded bytes.
// ============================================================================

const (
	// ExclusiveC14NAlgorithm is the canonicalization method URI to declare in
	// ds:CanonicalizationMethod/@Algorithm when signing with this package.
	ExclusiveC14NAlgorithm = "http://www.w3.org/2001/10/xml-exc-c14n#"

	xmlNamespaceURI = "http://www.w3.org/XML/1998/namespace"
)

type c14nConfig struct {
	withComments bool
}

// C14NOption configures CanonicalizeXML / Canonicalize.
type C14NOption func(*c14nConfig)

// WithComments includes XML comments in the canonical output (the
// "...#WithComments" variant). Omitted by default, matching the form used
// for XML-DSig digest computation.
func WithComments() C14NOption {
	return func(c *c14nConfig) { c.withComments = true }
}

// ---------------------------------------------------------------------------
// Internal node tree
// ---------------------------------------------------------------------------

type c14nKind int

const (
	c14nElem c14nKind = iota
	c14nText
	c14nComment
)

type c14nAttrNode struct {
	prefix string // literal prefix as written in the source, "" if none
	local  string
	nsURI  string // resolved namespace URI, "" if no namespace
	value  string
}

type c14nNode struct {
	kind     c14nKind
	prefix   string // element only: literal prefix as written in the source
	local    string // element only
	nsURI    string // element only: resolved namespace URI
	attrs    []c14nAttrNode
	children []*c14nNode
	text     string // text / comment only
}

// parseC14NTree re-parses raw XML bytes into a namespace-aware node tree.
// It uses Decoder.RawToken (not Token) specifically to keep the literal
// prefix strings from the source instead of Go's usual behavior of
// resolving them away to bare namespace URIs.
func parseC14NTree(xmlBytes []byte) (*c14nNode, error) {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))

	var root *c14nNode
	var stack []*c14nNode
	var scopes []map[string]string // stack-aligned: fully resolved prefix->URI in scope

	for {
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			scope := map[string]string{}
			if len(scopes) > 0 {
				for k, v := range scopes[len(scopes)-1] {
					scope[k] = v
				}
			}

			var realAttrs []xml.Attr
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == "xmlns":
					scope[a.Name.Local] = a.Value
				case a.Name.Space == "" && a.Name.Local == "xmlns":
					scope[""] = a.Value
				default:
					realAttrs = append(realAttrs, a)
				}
			}
			scopes = append(scopes, scope)

			resolve := func(prefix string) string {
				if prefix == "xml" {
					return xmlNamespaceURI
				}
				return scope[prefix]
			}

			node := &c14nNode{
				kind:   c14nElem,
				prefix: t.Name.Space,
				local:  t.Name.Local,
				nsURI:  resolve(t.Name.Space),
			}
			for _, a := range realAttrs {
				var ns string
				// Unprefixed attributes never inherit the default namespace.
				if a.Name.Space != "" {
					ns = resolve(a.Name.Space)
				}
				node.attrs = append(node.attrs, c14nAttrNode{
					prefix: a.Name.Space,
					local:  a.Name.Local,
					nsURI:  ns,
					value:  a.Value,
				})
			}

			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, node)
			} else if root == nil {
				root = node
			}
			stack = append(stack, node)

		case xml.EndElement:
			stack = stack[:len(stack)-1]
			scopes = scopes[:len(scopes)-1]

		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, &c14nNode{kind: c14nText, text: string(t)})

		case xml.Comment:
			if len(stack) == 0 {
				continue
			}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, &c14nNode{kind: c14nComment, text: string(t)})
		}
	}

	if root == nil {
		return nil, fmt.Errorf("c14n: no root element found")
	}
	return root, nil
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

type c14nNSDecl struct{ prefix, uri string }

// renderCanonicalNode writes node (treated as the root of its own
// canonicalization scope) into buf. rendered tracks, per this call, which
// (prefix -> uri) pairs have already been declared by an output ancestor;
// pass an empty map to canonicalize node as a standalone subtree.
func renderCanonicalNode(buf *bytes.Buffer, node *c14nNode, rendered map[string]string, cfg *c14nConfig) {
	// 1. Determine namespace declarations this element must (re)render.
	pending := map[string]string{}
	consider := func(prefix, uri string) {
		if prefix == "xml" {
			return
		}
		cur, ok := rendered[prefix]
		if !ok {
			cur = "" // never declared upstream == same as declared empty/no namespace
		}
		if cur == uri {
			return
		}
		pending[prefix] = uri
	}
	consider(node.prefix, node.nsURI)
	for _, a := range node.attrs {
		if a.prefix != "" {
			consider(a.prefix, a.nsURI)
		}
	}

	var needed []c14nNSDecl
	for p, u := range pending {
		needed = append(needed, c14nNSDecl{p, u})
	}
	sort.Slice(needed, func(i, j int) bool { return needed[i].prefix < needed[j].prefix })

	// 2. Sort attributes: unprefixed (no namespace) first by local name, then
	// namespace-qualified by (namespace URI, local name).
	sortedAttrs := append([]c14nAttrNode(nil), node.attrs...)
	sort.SliceStable(sortedAttrs, func(i, j int) bool {
		ai, aj := sortedAttrs[i], sortedAttrs[j]
		iNoNS := ai.prefix == ""
		jNoNS := aj.prefix == ""
		if iNoNS != jNoNS {
			return iNoNS
		}
		if iNoNS {
			return ai.local < aj.local
		}
		if ai.nsURI != aj.nsURI {
			return ai.nsURI < aj.nsURI
		}
		return ai.local < aj.local
	})

	// 3. Start tag.
	buf.WriteByte('<')
	writeQName(buf, node.prefix, node.local)
	for _, d := range needed {
		buf.WriteString(" xmlns")
		if d.prefix != "" {
			buf.WriteByte(':')
			buf.WriteString(d.prefix)
		}
		buf.WriteString(`="`)
		buf.WriteString(escapeAttr(d.uri))
		buf.WriteString(`"`)
	}
	for _, a := range sortedAttrs {
		buf.WriteByte(' ')
		writeQName(buf, a.prefix, a.local)
		buf.WriteString(`="`)
		buf.WriteString(escapeAttr(a.value))
		buf.WriteString(`"`)
	}
	buf.WriteByte('>')

	// 4. Children, with the namespace context extended by what we just rendered.
	childRendered := rendered
	if len(needed) > 0 {
		childRendered = make(map[string]string, len(rendered)+len(needed))
		for k, v := range rendered {
			childRendered[k] = v
		}
		for _, d := range needed {
			childRendered[d.prefix] = d.uri
		}
	}
	for _, c := range node.children {
		switch c.kind {
		case c14nElem:
			renderCanonicalNode(buf, c, childRendered, cfg)
		case c14nText:
			buf.WriteString(escapeText(c.text))
		case c14nComment:
			if cfg.withComments {
				buf.WriteString("<!--")
				buf.WriteString(c.text)
				buf.WriteString("-->")
			}
		}
	}

	// 5. Close tag (never self-closing).
	buf.WriteString("</")
	writeQName(buf, node.prefix, node.local)
	buf.WriteByte('>')
}

// ---------------------------------------------------------------------------
// Tree helpers (used by Signer.Verify to locate and re-canonicalize
// fragments of an already-parsed document without a marshal/reparse cycle).
// ---------------------------------------------------------------------------

// renderCanonicalized canonicalizes node as the root of its own scope (an
// "isolated" render — see the package doc comment on Exclusive C14N above).
func renderCanonicalized(node *c14nNode) ([]byte, error) {
	var buf bytes.Buffer
	renderCanonicalNode(&buf, node, map[string]string{}, &c14nConfig{})
	return buf.Bytes(), nil
}

// findElementNS searches node and its descendants (depth-first, node itself
// included) for the first element matching (nsURI, local).
func findElementNS(node *c14nNode, nsURI, local string) *c14nNode {
	if node == nil {
		return nil
	}
	if node.kind == c14nElem && node.nsURI == nsURI && node.local == local {
		return node
	}
	for _, c := range node.children {
		if c.kind == c14nElem {
			if found := findElementNS(c, nsURI, local); found != nil {
				return found
			}
		}
	}
	return nil
}

// findChildrenNS searches node's descendants (node itself excluded) for all
// elements matching (nsURI, local), depth-first, document order.
func findChildrenNS(node *c14nNode, nsURI, local string) []*c14nNode {
	var out []*c14nNode
	var walk func(*c14nNode)
	walk = func(n *c14nNode) {
		for _, c := range n.children {
			if c.kind != c14nElem {
				continue
			}
			if c.nsURI == nsURI && c.local == local {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk(node)
	return out
}

// findByID searches node and its descendants for the first element carrying
// an unprefixed "Id" attribute equal to id.
func findByID(node *c14nNode, id string) *c14nNode {
	if node == nil {
		return nil
	}
	if node.kind == c14nElem && attrValue(node, "Id") == id {
		return node
	}
	for _, c := range node.children {
		if c.kind == c14nElem {
			if found := findByID(c, id); found != nil {
				return found
			}
		}
	}
	return nil
}

// attrValue returns the value of the unprefixed attribute named local, or
// "" if not present.
func attrValue(node *c14nNode, local string) string {
	for _, a := range node.attrs {
		if a.prefix == "" && a.local == local {
			return a.value
		}
	}
	return ""
}

// nodeText concatenates and trims the direct text children of node.
func nodeText(node *c14nNode) string {
	var b strings.Builder
	for _, c := range node.children {
		if c.kind == c14nText {
			b.WriteString(c.text)
		}
	}
	return strings.TrimSpace(b.String())
}

// cloneWithoutFirst returns a deep copy of node's subtree with the first
// descendant-or-self element matching (nsURI, local) removed. Used to
// implement the enveloped-signature transform at verification time (the
// signer never needs this: its xmlContent is captured before the ds:
// Signature element exists at all).
func cloneWithoutFirst(node *c14nNode, nsURI, local string) *c14nNode {
	removed := false
	return cloneWithoutFirstRec(node, nsURI, local, &removed)
}

func cloneWithoutFirstRec(node *c14nNode, nsURI, local string, removed *bool) *c14nNode {
	clone := &c14nNode{
		kind:   node.kind,
		prefix: node.prefix,
		local:  node.local,
		nsURI:  node.nsURI,
		attrs:  node.attrs,
		text:   node.text,
	}
	for _, child := range node.children {
		if !*removed && child.kind == c14nElem && child.nsURI == nsURI && child.local == local {
			*removed = true
			continue
		}
		clone.children = append(clone.children, cloneWithoutFirstRec(child, nsURI, local, removed))
	}
	return clone
}

func writeQName(buf *bytes.Buffer, prefix, local string) {
	if prefix != "" {
		buf.WriteString(prefix)
		buf.WriteByte(':')
	}
	buf.WriteString(local)
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// CanonicalizeXML applies Exclusive XML Canonicalization directly to raw XML
// bytes, re-parsing them to resolve the namespace axis. Use this (rather
// than Canonicalize) when you already have serialized XML, to avoid an extra
// marshal round-trip.
func CanonicalizeXML(xmlBytes []byte, opts ...C14NOption) ([]byte, error) {
	cfg := &c14nConfig{}
	for _, o := range opts {
		o(cfg)
	}

	root, err := parseC14NTree(xmlBytes)
	if err != nil {
		return nil, fmt.Errorf("c14n: %w", err)
	}

	var buf bytes.Buffer
	renderCanonicalNode(&buf, root, map[string]string{}, cfg)
	return buf.Bytes(), nil
}

// Canonicalize serializes v (an *OrderedMap or map[string]any shaped like a
// document — a single root element key, same convention as Marshal/MapXML)
// and applies Exclusive XML Canonicalization to the result.
//
// This replaces the pre-v1.0.2 implementation, which walked OrderedMap
// directly and produced ad-hoc, non-spec output (no namespace axis, no
// distinction between namespace nodes and attributes, and — for maps that
// carried attributes at the very top level — dangling attribute text with no
// enclosing element at all). Callers must now pass a document-shaped map, as
// Marshal already requires everywhere else in this package.
func Canonicalize(v interface{}, opts ...C14NOption) ([]byte, error) {
	s, err := Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("c14n: marshal: %w", err)
	}
	return CanonicalizeXML([]byte(s), opts...)
}

// ---------------------------------------------------------------------------
// Escaping (C14N text/attribute-value rules)
// ---------------------------------------------------------------------------

func escapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\r", "&#xD;")
	return s
}

func escapeAttr(s string) string {
	s = escapeText(s)
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "\n", "&#xA;")
	s = strings.ReplaceAll(s, "\t", "&#x9;")
	return s
}
