package xml

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// OrderedMap is a hybrid data structure that maintains insertion order
// and offers advanced utilities for manipulating hierarchical data.
type OrderedMap struct {
	keys   []string       // Maintains the order
	values map[string]any // Maintains O(1) speed
}

// NewMap creates a new OrderedMap instance.
func NewMap() *OrderedMap {
	return &OrderedMap{
		keys:   make([]string, 0),
		values: make(map[string]any),
	}
}

// ---------------------------------------------------------
// 1. Fluent Setters & Core CRUD
// ---------------------------------------------------------

// Set inserts a value at a specific path (e.g. "Body/Auth/Token").
// It automatically creates the nested maps if they do not exist.
// Returns the same map for chaining (Fluent API).
func (om *OrderedMap) Set(path string, value any) *OrderedMap {
	parts := strings.Split(path, "/")
	lastIdx := len(parts) - 1
	current := om

	for i := 0; i < lastIdx; i++ {
		key := parts[i]
		// If it does not exist or is not an OrderedMap, create a new one
		if !current.Has(key) {
			newMap := NewMap()
			current.Put(key, newMap)
			current = newMap
		} else {
			// If it exists, verify that it is an OrderedMap
			val := current.Get(key)
			if nextMap, ok := val.(*OrderedMap); ok {
				current = nextMap
			} else {
				// Conflict: it exists but is not a map. Overwrite it.
				newMap := NewMap()
				current.Put(key, newMap)
				current = newMap
			}
		}
	}

	current.Put(parts[lastIdx], value)
	return om
}

// Put inserts a key-value pair directly at this level.
func (om *OrderedMap) Put(key string, value any) {
	if _, exists := om.values[key]; !exists {
		om.keys = append(om.keys, key)
	}
	om.values[key] = value
}

// Get retrieves a direct value from this level.
func (om *OrderedMap) Get(key string) any {
	return om.values[key]
}

// Has checks whether a key exists at this level.
func (om *OrderedMap) Has(key string) bool {
	_, exists := om.values[key]
	return exists
}

// Remove deletes a key while keeping the order consistent.
func (om *OrderedMap) Remove(key string) {
	if _, exists := om.values[key]; !exists {
		return
	}
	delete(om.values, key)
	for i, k := range om.keys {
		if k == key {
			om.keys = append(om.keys[:i], om.keys[i+1:]...)
			break
		}
	}
}

// Len returns the number of keys in this map.
func (om *OrderedMap) Len() int {
	return len(om.keys)
}

// ---------------------------------------------------------
// 2. Navigation and Safe Reading (Zero-Panic)
// ---------------------------------------------------------

// GetPath navigates the structure using "Key/SubKey".
func (om *OrderedMap) GetPath(path string) any {
	parts := strings.Split(path, "/")
	var current any = om

	for _, key := range parts {
		if omNode, ok := current.(*OrderedMap); ok {
			if !omNode.Has(key) {
				return nil
			}
			current = omNode.Get(key)
		} else if mapNode, ok := current.(map[string]any); ok {
			val, ok := mapNode[key]
			if !ok {
				return nil
			}
			current = val
		} else {
			return nil
		}
	}
	return current
}

// GetNode returns a *OrderedMap at the specified path.
// Returns nil if it does not exist or is not an OrderedMap.
func (om *OrderedMap) GetNode(path string) *OrderedMap {
	val := om.GetPath(path)
	if node, ok := val.(*OrderedMap); ok {
		return node
	}
	return nil // Returns nil, no panic
}

// List returns a slice of *OrderedMap.
// It is "smart": if the node is a single one, it wraps it in a slice.
// If it is a slice of any, it filters the ones that are OrderedMap.
func (om *OrderedMap) List(path string) []*OrderedMap {
	val := om.GetPath(path)
	if val == nil {
		return []*OrderedMap{}
	}

	result := make([]*OrderedMap, 0)

	switch v := val.(type) {
	case *OrderedMap:
		result = append(result, v)
	case []*OrderedMap:
		return v
	case []any:
		for _, item := range v {
			if node, ok := item.(*OrderedMap); ok {
				result = append(result, node)
			}
		}
	}
	return result
}

// String gets a string from the path, returning "" on failure.
func (om *OrderedMap) String(path string) string {
	val := om.GetPath(path)
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Int gets an int from the path, returning 0 on failure.
func (om *OrderedMap) Int(path string) int {
	val := om.GetPath(path)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

// Float gets a float64 from the path, returning 0.0 on failure.
func (om *OrderedMap) Float(path string) float64 {
	val := om.GetPath(path)
	if val == nil {
		return 0.0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0.0
	}
}

// Bool gets a bool from the path, returning false on failure.
func (om *OrderedMap) Bool(path string) bool {
	val := om.GetPath(path)
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(v)
		return s == "true" || s == "1" || s == "yes" || s == "on"
	case int:
		return v == 1
	default:
		return false
	}
}

// ---------------------------------------------------------
// 3. Utils & Iteration
// ---------------------------------------------------------

// Keys returns the keys in the current order.
func (om *OrderedMap) Keys() []string {
	result := make([]string, len(om.keys))
	copy(result, om.keys)
	return result
}

// Sort sorts the keys alphabetically.
func (om *OrderedMap) Sort() {
	sort.Strings(om.keys)
}

// ForEach iterates over the map in order.
func (om *OrderedMap) ForEach(fn func(key string, value any) bool) {
	for _, k := range om.keys {
		if !fn(k, om.values[k]) {
			break
		}
	}
}

// ToMap converts recursively to map[string]any (loses order).
func (om *OrderedMap) ToMap() map[string]any {
	result := make(map[string]any, len(om.keys))
	for _, k := range om.keys {
		val := om.values[k]
		result[k] = toNative(val)
	}
	return result
}

// Recursive helper for ToMap
func toNative(val any) any {
	switch v := val.(type) {
	case *OrderedMap:
		return v.ToMap()
	case []*OrderedMap:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = item.ToMap()
		}
		return list
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = toNative(item)
		}
		return list
	default:
		return v
	}
}

// ToJSON returns the JSON representation of the map preserving order (indirectly via MarshalJSON).
func (om *OrderedMap) ToJSON() (string, error) {
	b, err := om.MarshalJSON()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (om *OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range om.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyBytes, _ := json.Marshal(k)
		buf.Write(keyBytes)
		buf.WriteByte(':')
		valBytes, err := json.Marshal(om.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(valBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// ---------------------------------------------------------
// 4. XML Interoperability (The missing piece)
// ---------------------------------------------------------

// MarshalXML implements the xml.Marshaler interface.
// This allows OrderedMap to work natively with encoding/xml.
func (om *OrderedMap) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	var childrenKeys []string

	// 1. Separate Attributes (@) from Children
	// It is vital to inject the attributes into the 'start' token BEFORE emitting it.
	finalStart := start.Copy()

	for _, k := range om.keys {
		val := om.values[k]

		if strings.HasPrefix(k, "@") {
			// It is an attribute
			attrName := strings.TrimPrefix(k, "@")
			finalStart.Attr = append(finalStart.Attr, xml.Attr{
				Name:  xml.Name{Local: attrName},
				Value: fmt.Sprintf("%v", val),
			})
		} else {
			// It is content
			childrenKeys = append(childrenKeys, k)
		}
	}

	// 2. Emit Start Element (with attributes)
	if err := e.EncodeToken(finalStart); err != nil {
		return err
	}

	// 3. Emit Children (In Order)
	for _, k := range childrenKeys {
		val := om.values[k]

		if k == "#text" {
			// Direct text
			if err := e.EncodeToken(xml.CharData([]byte(fmt.Sprintf("%v", val)))); err != nil {
				return err
			}
			continue
		}

		// Normal child (Automatic recursion handled by Go)
		if err := e.EncodeElement(val, xml.StartElement{Name: xml.Name{Local: k}}); err != nil {
			return err
		}
	}

	// 4. Close Element
	return e.EncodeToken(finalStart.End())
}

// ---------------------------------------------------------
// 5. Debug Helper
// ---------------------------------------------------------

// Dump returns a pretty string representation (JSON Indented) of the structure.
// Useful for logs: fmt.Println(resp.Dump())
func (om *OrderedMap) Dump() string {
	b, err := om.MarshalJSON()
	if err != nil {
		return fmt.Sprintf("<DumpError: %v>", err)
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		return string(b) // Fallback to minified JSON if indentation fails
	}
	return out.String()
}

// ---------------------------------------------------------
// 6. Mutation (Rename / Move)
// ---------------------------------------------------------

// Rename changes the name of a key while keeping its position and value.
func (om *OrderedMap) Rename(oldKey, newKey string) error {
	if _, exists := om.values[newKey]; exists {
		return fmt.Errorf("destination key '%s' already exists", newKey)
	}
	val, exists := om.values[oldKey]
	if !exists {
		return fmt.Errorf("source key '%s' not found", oldKey)
	}

	// Update Map
	delete(om.values, oldKey)
	om.values[newKey] = val

	// Update Order
	for i, k := range om.keys {
		if k == oldKey {
			om.keys[i] = newKey
			return nil
		}
	}
	return nil
}

// Move moves a value from one path to another (Cut & Paste).
func (om *OrderedMap) Move(srcPath, dstPath string) error {
	val := om.GetPath(srcPath)
	if val == nil {
		return fmt.Errorf("source path '%s' not found", srcPath)
	}

	// 1. Paste (Set creates deep paths if necessary)
	om.Set(dstPath, val)

	// 2. Cut (Remove Source)
	parts := strings.Split(srcPath, "/")
	if len(parts) == 1 {
		om.Remove(parts[0])
	} else {
		parentPath := strings.Join(parts[:len(parts)-1], "/")
		childKey := parts[len(parts)-1]

		parent := om.GetNode(parentPath)
		if parent != nil {
			parent.Remove(childKey)
		}
	}
	return nil
}
