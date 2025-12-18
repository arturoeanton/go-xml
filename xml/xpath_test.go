package xml

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestXPath_Lite(t *testing.T) {
	data := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{
					"category": "reference",
					"author":   "Nigel Rees",
					"title":    "Sayings of the Century",
					"price":    8.95,
				},
				map[string]any{
					"category": "fiction",
					"author":   "Evelyn Waugh",
					"title":    "Sword of Honour",
					"price":    12.99,
				},
				map[string]any{
					"category": "fiction",
					"author":   "Herman Melville",
					"title":    "Moby Dick",
					"isbn":     "0-553-21311-3",
					"price":    8.99,
				},
				map[string]any{
					"category": "fiction",
					"author":   "J. R. R. Tolkien",
					"title":    "The Lord of the Rings",
					"isbn":     "0-395-19395-8",
					"price":    22.99,
				},
			},
			"bicycle": map[string]any{
				"color": "red",
				"price": 19.95,
			},
		},
		"errors": map[string]any{
			"log": []any{
				map[string]any{"level": "error", "msg": "DB fail"},
				map[string]any{"level": "info", "msg": "App started"},
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected []any
	}{
		// 1. #count
		{
			name:     "Count books",
			path:     "store/book/#count",
			expected: []any{4},
		},
		{
			name:     "Count dictionary",
			path:     "store/bicycle/#count",
			expected: []any{2}, // color and price
		},

		// 2. Operators
		{
			name:     "Price < 10",
			path:     "store/book[price<10]/title",
			expected: []any{"Sayings of the Century", "Moby Dick"},
		},
		{
			name:     "Price > 20",
			path:     "store/book[price>20]/title",
			expected: []any{"The Lord of the Rings"},
		},
		{
			name:     "Category != fiction",
			path:     "store/book[category!=fiction]/title",
			expected: []any{"Sayings of the Century"},
		},

		// 3. Functions
		{
			name:     "Contains 'Moby'",
			path:     "store/book[contains(title, 'Moby')]/title",
			expected: []any{"Moby Dick"},
		},
		{
			name:     "Starts with 'Evelyn'",
			path:     "store/book[starts-with(author, 'Evelyn')]/title",
			expected: []any{"Sword of Honour"},
		},

		// 4. Deep Search //
		{
			name:     "Deep search title",
			path:     "//title",
			expected: []any{"Sayings of the Century", "Sword of Honour", "Moby Dick", "The Lord of the Rings"},
		},
		{
			name:     "Deep search price",
			path:     "//price",
			expected: []any{8.95, 12.99, 8.99, 22.99, 19.95}, // books + bicycle
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := QueryAll(data, tt.path)
			if err != nil {
				t.Fatalf("QueryAll error: %v", err)
			}

			// Normalize results for comparison (order might vary for maps/deep search)
			gotStrs := stringifyList(results)
			expStrs := stringifyList(tt.expected)

			sort.Strings(gotStrs)
			sort.Strings(expStrs)

			if !reflect.DeepEqual(gotStrs, expStrs) {
				t.Errorf("QueryAll(%q) = %v; want %v", tt.path, gotStrs, expStrs)
			}
		})
	}
}

func stringifyList(list []any) []string {
	var strs []string
	for _, v := range list {
		// Use fmt.Sprint for generic string representation
		strs = append(strs, fmt.Sprint(v))
	}
	return strs
}
