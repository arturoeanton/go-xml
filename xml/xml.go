package xml

import (
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

// ============================================================================
// 1. CONFIGURATION AND OPTIONS
// ============================================================================

type config struct {
	forceArrayKeys map[string]bool             // Tags that will always be treated as a list
	namespaces     map[string]string           // Namespace Aliases
	valueHooks     map[string]func(string) any // Transformation Hooks

	// Flags
	isLenient        bool // Tolerant mode for dirty HTML/XML
	inferTypes       bool // Automatic type inference (int, bool, float)
	isSoupMode       bool // "Soup Mode" (Dirty HTML - Normalization & Sanitization)
	useCharsetReader bool // Use charset reader for ISO-8859-1 and Windows-1252
	prettyPrint      bool // Indentation output
	htmlAutoClose    []string
}

type Option func(*config)

func defaultConfig() *config {
	return &config{
		forceArrayKeys:   make(map[string]bool),
		namespaces:       make(map[string]string),
		valueHooks:       make(map[string]func(string) any),
		useCharsetReader: false,
	}
}

// ForceArray forces specific tags to be parsed as arrays ([]any).
func ForceArray(keys ...string) Option {
	return func(c *config) {
		for _, k := range keys {
			c.forceArrayKeys[k] = true
		}
	}
}

// EnableLegacyCharsets enables ISO-8859-1 support.
func EnableLegacyCharsets() Option {
	return func(c *config) {
		c.useCharsetReader = true
	}
}

// RegisterNamespace registers a short alias for a namespace URL.
func RegisterNamespace(alias, url string) Option {
	return func(c *config) { c.namespaces[url] = alias }
}

// WithValueHook registers a transformation function for a tag.
func WithValueHook(tagName string, fn func(string) any) Option {
	return func(c *config) { c.valueHooks[tagName] = fn }
}

// EnableExperimental enables Soup Mode (for dirty HTML).
func EnableExperimental() Option {
	return func(c *config) {
		c.inferTypes = true
		c.isLenient = true
		c.isSoupMode = true
		c.htmlAutoClose = []string{
			"area", "base", "br", "col", "embed", "hr", "img", "input",
			"link", "meta", "param", "source", "track", "wbr",
			"command", "keygen", "menuitem",
		}
	}
}

// WithPrettyPrint enables indentation for the Encoder.
func WithPrettyPrint() Option {
	return func(c *config) { c.prettyPrint = true }
}

// ============================================================================
// 2. PARSER CORE
// ============================================================================

type node struct {
	tagName string
	data    *OrderedMap
}

// MapXML reads XML into a deterministic OrderedMap.
func MapXML(r io.Reader, opts ...Option) (*OrderedMap, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.isSoupMode {
		r = sanitizeSoup(r)
	}

	decoder := xml.NewDecoder(r)
	if cfg.isLenient {
		decoder.Strict = false
		decoder.AutoClose = cfg.htmlAutoClose
		decoder.Entity = xml.HTMLEntity
	}
	if cfg.useCharsetReader {
		decoder.CharsetReader = charsetReader
	}

	root := NewMap()
	stack := []*node{{tagName: "", data: root}}

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			if cfg.isSoupMode {
				continue
			}
			return nil, err
		}

		switch se := token.(type) {
		case xml.StartElement:
			localName := se.Name.Local
			if cfg.isSoupMode {
				localName = strings.ToLower(localName)
			}
			tagName := resolveName(xml.Name{Space: se.Name.Space, Local: localName}, cfg.namespaces)

			currentMap := NewMap()

			// Process Attributes
			for _, attr := range se.Attr {
				attrName := attr.Name.Local
				if cfg.isSoupMode {
					attrName = strings.ToLower(attrName)
				}
				attrName = resolveName(xml.Name{Space: attr.Name.Space, Local: attrName}, cfg.namespaces)
				currentMap.Put("@"+attrName, processValue(attr.Value, "", cfg))
			}

			stack = append(stack, &node{tagName: tagName, data: currentMap})

		case xml.CharData:
			content := string(se)
			trimmed := strings.TrimSpace(content)

			// Only process significant content
			if trimmed != "" {
				current := stack[len(stack)-1]

				// #text accumulation
				if existingText := current.data.Get("#text"); existingText != nil {
					current.data.Put("#text", existingText.(string)+trimmed)
				} else {
					current.data.Put("#text", trimmed)
				}
			}

		case xml.EndElement:
			childNode := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			if len(stack) == 0 {
				continue
			}

			parent := stack[len(stack)-1]
			tagName := childNode.tagName

			// Node Simplification
			var finalValue any = childNode.data
			if childNode.data.Len() == 1 && childNode.data.Has("#text") {
				finalValue = processValue(childNode.data.Get("#text").(string), tagName, cfg)
			}

			// Add to Parent
			existingValue := parent.data.Get(tagName)
			if existingValue == nil {
				if cfg.forceArrayKeys[tagName] {
					parent.data.Put(tagName, []any{finalValue})
				} else {
					parent.data.Put(tagName, finalValue)
				}
			} else {
				if list, ok := existingValue.([]any); ok {
					parent.data.Put(tagName, append(list, finalValue))
				} else {
					parent.data.Put(tagName, []any{existingValue, finalValue})
				}
			}
		}
	}

	return root, nil
}

func resolveName(name xml.Name, nsMap map[string]string) string {
	if alias, ok := nsMap[name.Space]; ok && alias != "" {
		return alias + ":" + name.Local
	}
	return name.Local
}

func processValue(val string, tagName string, cfg *config) any {
	if hook, ok := cfg.valueHooks[tagName]; ok {
		return hook(val)
	}
	if cfg.inferTypes {
		return inferType(val)
	}
	return val
}

func inferType(val string) any {
	if val == "true" {
		return true
	}
	if val == "false" {
		return false
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil && strings.Contains(val, ".") {
		return f
	}
	return val
}
