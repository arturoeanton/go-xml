package xml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ============================================================================
// STREAMING ENCODER
// ============================================================================

// Encoder writes XML directly to an io.Writer.
type Encoder struct {
	w   io.Writer
	cfg *config
}

// NewEncoder creates a configured encoder.
func NewEncoder(w io.Writer, opts ...Option) *Encoder {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &Encoder{w: w, cfg: cfg}
}

// Encode writes the map data as XML.
func (e *Encoder) Encode(data any) error {
	var keys []string
	var valGetter func(string) any

	// Strategy: Determine if OrderedMap or Standard Map
	if om, ok := data.(*OrderedMap); ok {
		keys = om.Keys()
		valGetter = om.Get
	} else if m, ok := data.(map[string]any); ok {
		keys = sortedKeys(m)
		valGetter = func(k string) any { return m[k] }
	} else {
		return fmt.Errorf("unsupported type for Encode: %T. Expected *OrderedMap or map[string]any", data)
	}

	// Validation: Root must have exactly 1 element
	rootTag := ""
	for _, k := range keys {
		if !strings.HasPrefix(k, "#") && !strings.HasPrefix(k, "@") {
			if rootTag != "" {
				return errors.New("root must have exactly 1 element")
			}
			rootTag = k
		}
	}
	if rootTag == "" {
		return errors.New("root element not found")
	}

	val := valGetter(rootTag)
	return encodeNode(e.w, rootTag, val, e.cfg, 0)
}

// Marshal returns the XML as a string (Helper wrapper).
func Marshal(data any, opts ...Option) (string, error) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf, opts...)
	if err := enc.Encode(data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// encodeNode writes a single node recursively.
func encodeNode(w io.Writer, tag string, value any, cfg *config, depth int) error {
	indent := ""
	if cfg.prettyPrint {
		indent = "\n" + strings.Repeat("  ", depth)
	}

	// Prepare Start Element
	startElem := "<" + tag

	// Handle Namespaces (only at Root / depth 0)
	if depth == 0 && len(cfg.namespaces) > 0 {
		var urls []string
		for u := range cfg.namespaces {
			urls = append(urls, u)
		}
		sort.Strings(urls)
		for _, u := range urls {
			alias := cfg.namespaces[u]
			startElem += fmt.Sprintf(` xmlns:%s="%s"`, alias, u)
		}
	}

	var content any
	var cdataContent string
	var childrenKeys []string
	var childrenValGetter func(string) any
	var isComplex bool

	// Extract Attributes and Children
	switch v := value.(type) {
	case *OrderedMap:
		isComplex = true
		allKeys := v.Keys()
		childrenValGetter = v.Get

		// 1. Filter Attributes
		for _, k := range allKeys {
			if strings.HasPrefix(k, "@") {
				val := v.Get(k)
				esc := escapeString(fmt.Sprintf("%v", val))
				startElem += fmt.Sprintf(` %s="%s"`, strings.TrimPrefix(k, "@"), esc)
			} else if k == "#text" {
				content = v.Get(k)
			} else if k == "#cdata" {
				cdataContent = fmt.Sprintf("%v", v.Get(k))
			} else {
				childrenKeys = append(childrenKeys, k)
			}
		}

	case map[string]any:
		isComplex = true
		allKeys := sortedKeys(v)
		childrenValGetter = func(k string) any { return v[k] }

		for _, k := range allKeys {
			if strings.HasPrefix(k, "@") {
				val := v[k]
				esc := escapeString(fmt.Sprintf("%v", val))
				startElem += fmt.Sprintf(` %s="%s"`, strings.TrimPrefix(k, "@"), esc)
			} else if k == "#text" {
				content = v[k]
			} else if k == "#cdata" {
				cdataContent = fmt.Sprintf("%v", v[k])
			} else {
				childrenKeys = append(childrenKeys, k)
			}
		}

	default:
		// Simple Value
		content = v
	}

	startElem += ">"
	fmt.Fprint(w, indent+startElem)

	// Write Content
	if cdataContent != "" {
		fmt.Fprint(w, "<![CDATA["+cdataContent+"]]>")
	} else if content != nil {
		xml.EscapeText(w, []byte(fmt.Sprintf("%v", content)))
	}

	// Write Children
	if isComplex {
		for _, k := range childrenKeys {
			childVal := childrenValGetter(k)

			// Recursion
			// Handle Arrays
			if list, ok := childVal.([]any); ok {
				for _, item := range list {
					if err := encodeNode(w, k, item, cfg, depth+1); err != nil {
						return err
					}
				}
			} else if omList, ok := childVal.([]*OrderedMap); ok {
				for _, item := range omList {
					if err := encodeNode(w, k, item, cfg, depth+1); err != nil {
						return err
					}
				}
			} else {
				if err := encodeNode(w, k, childVal, cfg, depth+1); err != nil {
					return err
				}
			}
		}
	}

	if isComplex && cfg.prettyPrint {
		fmt.Fprint(w, "\n"+strings.Repeat("  ", depth))
	}
	fmt.Fprint(w, "</"+tag+">")
	return nil
}

// Helpers
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func escapeString(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
