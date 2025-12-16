package xml

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// 1. SERIALIZATION & BINDING
// ============================================================================

// ToJSON converts the XML map into a JSON string.
// This helper is particularly useful for debugging purposes or for preparing API responses.
func ToJSON(data map[string]any) (string, error) {
	b, err := json.Marshal(data)
	return string(b), err
}

// MapToStruct converts the dynamic map into a user-defined struct.
// It uses JSON serialization as an intermediate layer, which is the cleanest
// and most robust approach to map dynamic keys to struct fields (respecting tags).
func MapToStruct(data map[string]any, result any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, result)
}

// ============================================================================
// 2. TYPE COERCION UTILITIES (SAFE GETTERS)
// ============================================================================

// AsString safely converts any value to a string.
// It handles primitives, maps (JSON representation), and nil (empty string).
func AsString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case fmt.Stringer:
		return t.String()
	case error:
		return t.Error()
	}
	// Fallback to JSON or standard formatting
	if reflect.TypeOf(v).Kind() == reflect.Map || reflect.TypeOf(v).Kind() == reflect.Slice {
		b, _ := json.Marshal(v)
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

// AsInt attempts to convert any value to an int.
// Returns 0 if conversion fails. Supports strings like "123", floats (truncated), and bools (1/0).
func AsInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case float64:
		return int(t)
	case bool:
		if t {
			return 1
		}
		return 0
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(t))
		return i
	}
	return 0
}

// AsFloat attempts to convert any value to a float64.
// Returns 0.0 if conversion fails.
func AsFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	}
	return 0.0
}

// AsBool parses a boolean leniently.
// True: true, "true", "1", "yes", "on", "ok".
// False: everything else.
func AsBool(v any) bool {
	s := strings.ToLower(fmt.Sprintf("%v", v))
	return s == "true" || s == "1" || s == "yes" || s == "on" || s == "ok"
}

// AsTime parses a string into time.Time using a list of potential layouts.
// Default layouts: RFC3339, YYYY-MM-DD, and generic SQL timestamps.
func AsTime(v any, layouts ...string) (time.Time, error) {
	s := AsString(v)
	if len(layouts) == 0 {
		layouts = []string{
			time.RFC3339,
			"2006-01-02",
			"2006-01-02 15:04:05",
			time.RFC1123,
		}
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

// AsSlice guarantees the return of a []any.
// If the input is nil, returns empty slice.
// If the input is a single element, returns a slice with that element.
func AsSlice(v any) []any {
	if v == nil {
		return []any{}
	}
	if list, ok := v.([]any); ok {
		return list
	}
	return []any{v}
}

// ============================================================================
// 3. MAP INSPECTION & FILTERING
// ============================================================================

// Keys returns a sorted list of all keys in the map.
func Keys(data map[string]any) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Attributes returns only the keys that represent XML attributes (starting with "@").
func Attributes(data map[string]any) map[string]any {
	attrs := make(map[string]any)
	for k, v := range data {
		if strings.HasPrefix(k, "@") {
			attrs[k] = v
		}
	}
	return attrs
}

// Children returns only the keys that represent child nodes (excluding attributes and #text).
func Children(data map[string]any) map[string]any {
	children := make(map[string]any)
	for k, v := range data {
		if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
			children[k] = v
		}
	}
	return children
}

// Has checks if a key exists in the map (shallow check).
func Has(data map[string]any, key string) bool {
	_, exists := data[key]
	return exists
}

// Pick returns a new map containing only the specified keys.
func Pick(data map[string]any, keys ...string) map[string]any {
	result := make(map[string]any)
	for _, k := range keys {
		if v, ok := data[k]; ok {
			result[k] = v
		}
	}
	return result
}

// Omit returns a new map excluding the specified keys.
func Omit(data map[string]any, keys ...string) map[string]any {
	result := make(map[string]any)
	ignored := make(map[string]bool)
	for _, k := range keys {
		ignored[k] = true
	}
	for k, v := range data {
		if !ignored[k] {
			result[k] = v
		}
	}
	return result
}

// ============================================================================
// 4. STRUCTURAL TRANSFORMATION (MERGE, CLONE, FLATTEN)
// ============================================================================

// Clone creates a deep copy of the map, ensuring no reference sharing.
// Crucial for immutable operations or concurrent access.
func Clone(data map[string]any) map[string]any {
	// JSON marshaling is the laziest but most robust way to deep clone generic maps
	b, _ := json.Marshal(data)
	var copy map[string]any
	json.Unmarshal(b, &copy)
	return copy
}

// Merge recursively merges the 'override' map into the 'base' map.
// If a key exists in both maps and they are sub-maps, they are merged recursively.
// Otherwise, the value in 'base' is overwritten by the value from 'override'.
// Note: This modifies 'base' in place.
func Merge(base, override map[string]any) {
	for k, v := range override {
		// If both are maps, merge recursively
		if vMap, ok := v.(map[string]any); ok {
			if baseMap, ok := base[k].(map[string]any); ok {
				Merge(baseMap, vMap)
				continue
			}
		}
		// Otherwise, overwrite
		base[k] = v
	}
}

// MergeDeep is an alias for Merge (included for API clarity regarding deep merging).
func MergeDeep(base, override map[string]any) {
	Merge(base, override)
}

// Flatten converts a nested map into a single-level map with dot notation.
// Example: {"a": {"b": 1}} -> {"a.b": 1}
// Useful for exporting to CSV or searching.
func Flatten(data map[string]any) map[string]any {
	result := make(map[string]any)
	flattenRecursive(data, "", result)
	return result
}

func flattenRecursive(data map[string]any, prefix string, result map[string]any) {
	for k, v := range data {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if vMap, ok := v.(map[string]any); ok {
			flattenRecursive(vMap, key, result)
		} else {
			result[key] = v
		}
	}
}

// Text extracts ALL text content recursively from a node and its children.
// Equivalent to jQuery's .text(). Useful for search indexing.
func Text(data any) string {
	var sb strings.Builder
	textRecursive(data, &sb)
	return strings.TrimSpace(sb.String())
}

func textRecursive(data any, sb *strings.Builder) {
	if data == nil {
		return
	}

	switch v := data.(type) {
	case string:
		sb.WriteString(v) // Sin espacio forzado aquí, el parser ya trae espacios si es HTML

	case int, float64, bool:
		sb.WriteString(fmt.Sprintf("%v", v))

	case map[string]any:
		// ESTRATEGIA 1: Si existe #seq, ES LA FUENTE DE LA VERDAD (Ordenada)
		if seq, ok := v["#seq"].([]any); ok {
			for _, item := range seq {
				textRecursive(item, sb)
			}
			return // Terminamos aquí, no miramos keys ni #text
		}

		// ESTRATEGIA 2: Fallback (Legacy / Simplificado)
		if t, ok := v["#text"]; ok {
			sb.WriteString(fmt.Sprintf("%v", t))
		}

		// Iterar hijos (alfabéticamente, no garantiza orden de lectura)
		keys := Keys(v)
		for _, k := range keys {
			if !strings.HasPrefix(k, "@") && k != "#text" && k != "#seq" {
				textRecursive(v[k], sb)
			}
		}

	case []any:
		for _, item := range v {
			textRecursive(item, sb)
		}
	}
}

// ============================================================================
// 5. FUNCTIONAL HELPERS (LIST PROCESSING)
// ============================================================================

// MapSlice applies a transformation function to each element in a slice.
func MapSlice[T any, R any](input []T, transform func(T) R) []R {
	result := make([]R, len(input))
	for i, v := range input {
		result[i] = transform(v)
	}
	return result
}

// FilterSlice returns a new slice containing only elements that satisfy the predicate.
func FilterSlice[T any](input []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range input {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// FindFirst returns the first element satisfying the predicate, or zero value if not found.
func FindFirst[T any](input []T, predicate func(T) bool) (T, bool) {
	for _, v := range input {
		if predicate(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}
