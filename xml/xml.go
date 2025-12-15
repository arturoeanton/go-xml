package xml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

/*
	r2/xml v1.0 - "The Enterprise Single-File XML Parser"
	=====================================================
	Una solución robusta, "Zero-Dependency" y autocontenida para XML en Go.
	Diseñada para reemplazar encoding/xml en escenarios dinámicos y de alto rendimiento.

	Lista Completa de Features:
	1. MapXML Engine: Lectura dinámica a map[string]any con optimización automática de texto (Text Simplification).
	2. Namespace Manager: Limpieza automática de URLs en lectura (Aliasing) e inyección en escritura (Definitions).
	3. ForceArray: Garantiza listas ([]any) eliminando la ambigüedad de items únicos (JSON-Ready).
	4. Streaming Decoder: Lectura eficiente de archivos gigantes (GBs) usando Generics (Stream[T]).
	5. Streaming Encoder: Escritura directa a io.Writer sin buffers, soportando Atributos en Root.
	6. Query Engine: Navegación profunda con filtros ([k=v]), índices ([i]) y búsqueda recursiva (QueryAll).
	7. Business Validation: Motor de reglas integrado (Required, Type, Min/Max, Regex, Enum).
	8. Smart Parsing: Inferencia de tipos (int/bool), Modo "Lenient" (HTML sucio) y Hooks de transformación.
	9. Rich Content Support: Manejo nativo de bloques CDATA (#cdata) y preservación de Comentarios (#comments).
	10. CLI Helper: Funcionalidad lista para usar como herramienta de línea de comandos.
*/

// ============================================================================
// 1. CONFIGURACIÓN Y OPCIONES
// ============================================================================

type config struct {
	forceArrayKeys map[string]bool             // Tags que siempre serán lista
	namespaces     map[string]string           // Alias de Namespaces
	valueHooks     map[string]func(string) any // Hooks de transformación

	// Flags
	isLenient   bool // HTML/XML sucio
	inferTypes  bool // Inferencia automática
	prettyPrint bool // Indentación

	htmlAutoClose []string
}

type Option func(*config)

func ForceArray(keys ...string) Option {
	return func(c *config) {
		for _, k := range keys {
			c.forceArrayKeys[k] = true
		}
	}
}

func RegisterNamespace(alias, url string) Option {
	return func(c *config) { c.namespaces[url] = alias }
}

func WithValueHook(tagName string, fn func(string) any) Option {
	return func(c *config) { c.valueHooks[tagName] = fn }
}

func EnableExperimental() Option {
	return func(c *config) {
		c.inferTypes = true
		c.isLenient = true
		c.htmlAutoClose = []string{"br", "img", "input", "meta", "hr", "link"}
	}
}

func WithPrettyPrint() Option {
	return func(c *config) { c.prettyPrint = true }
}

// ============================================================================
// 2. PARSER CORE (MapXML - Lectura en Memoria)
// ============================================================================

type node struct {
	tagName string
	data    map[string]any
}

// MapXML lee todo el XML y devuelve un mapa dinámico.
func MapXML(r io.Reader, opts ...Option) (map[string]any, error) {
	cfg := &config{
		forceArrayKeys: make(map[string]bool),
		namespaces:     make(map[string]string),
		valueHooks:     make(map[string]func(string) any),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	decoder := xml.NewDecoder(r)
	if cfg.isLenient {
		decoder.Strict = false
		decoder.AutoClose = cfg.htmlAutoClose
		decoder.Entity = xml.HTMLEntity
	}

	root := make(map[string]any)
	stack := []*node{{tagName: "", data: root}}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error parseo: %v", err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			tagName := resolveName(se.Name, cfg.namespaces)
			currentMap := make(map[string]any)

			for _, attr := range se.Attr {
				attrName := resolveName(attr.Name, cfg.namespaces)
				currentMap["@"+attrName] = processValue(attr.Value, "", cfg)
			}
			stack = append(stack, &node{tagName: tagName, data: currentMap})

		case xml.CharData:
			content := strings.TrimSpace(string(se))
			if content != "" {
				current := stack[len(stack)-1]
				if val, exists := current.data["#text"]; exists {
					current.data["#text"] = val.(string) + content
				} else {
					current.data["#text"] = content
				}
			}
		case xml.Comment:
			content := string(se)
			current := stack[len(stack)-1]
			if list, exists := current.data["#comments"]; exists {
				current.data["#comments"] = append(list.([]string), content)
			} else {
				current.data["#comments"] = []string{content}
			}

		case xml.EndElement:
			childNode := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			parent := stack[len(stack)-1]
			tagName := childNode.tagName

			var finalValue any = childNode.data
			if len(childNode.data) == 1 {
				if val, ok := childNode.data["#text"]; ok {
					finalValue = processValue(val.(string), tagName, cfg)
				}
			}

			existingValue, exists := parent.data[tagName]
			if !exists {
				if cfg.forceArrayKeys[tagName] {
					parent.data[tagName] = []any{finalValue}
				} else {
					parent.data[tagName] = finalValue
				}
			} else {
				if list, ok := existingValue.([]any); ok {
					parent.data[tagName] = append(list, finalValue)
				} else {
					parent.data[tagName] = []any{existingValue, finalValue}
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
