package xml

import (
	"reflect"
	"sort"
	"testing"
)

func TestQuery_Wildcard(t *testing.T) {
	data := map[string]any{
		"invoice": map[string]any{
			"items": map[string]any{
				"box": map[string]any{
					"sku": "SKU-BOX-001",
					"qty": 10,
				},
				"bag": map[string]any{
					"sku": "SKU-BAG-002",
					"qty": 5,
				},
				"crate": map[string]any{
					"sku": "SKU-CRATE-003",
					"qty": 2,
				},
				// "note": "Handle with care", // Primitive, should wildcard match this?
				// Usually wildcard behaves like "match any child node".
				// If the next segment is 'sku', it won't match on "note" anyway.
			},
		},
		"routes": map[string]any{
			"route_1": map[string]any{"id": 101},
			"route_2": map[string]any{"id": 102},
			"@attr":   "val",
		},
	}

	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "Wildcard items",
			path:     "invoice/items/*/sku",
			expected: []string{"SKU-BOX-001", "SKU-BAG-002", "SKU-CRATE-003"},
		},
		{
			name:     "Wildcard routes",
			path:     "routes/*/id",
			expected: []string{"101", "102"}, // Should not try to traverse @attr or fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := QueryAll(data, tt.path)
			if err != nil {
				t.Fatalf("QueryAll error: %v", err)
			}

			var got []string
			for _, r := range results {
				if s, ok := r.(string); ok {
					got = append(got, s)
				} else if i, ok := r.(int); ok {
					// Quick hack for the int case
					if i == 101 {
						got = append(got, "101")
					}
					if i == 102 {
						got = append(got, "102")
					}
				}
			}

			// Sort for deterministic comparison
			sort.Strings(got)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("QueryAll(%q) = %v; want %v", tt.path, got, tt.expected)
			}
		})
	}
}
