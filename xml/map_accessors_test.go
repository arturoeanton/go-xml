package xml

import "testing"

func TestOrderedMap_Float(t *testing.T) {
	m := NewMap()
	m.Put("f", 3.14)
	m.Put("i", 5)
	m.Put("s", "2.5")
	m.Put("bad", "not-a-number")

	if got := m.Float("f"); got != 3.14 {
		t.Errorf("Float(float64) = %v, want 3.14", got)
	}
	if got := m.Float("i"); got != 5.0 {
		t.Errorf("Float(int) = %v, want 5.0", got)
	}
	if got := m.Float("s"); got != 2.5 {
		t.Errorf("Float(string) = %v, want 2.5", got)
	}
	if got := m.Float("bad"); got != 0.0 {
		t.Errorf("Float(invalid string) = %v, want 0.0", got)
	}
	if got := m.Float("missing"); got != 0.0 {
		t.Errorf("Float(missing) = %v, want 0.0", got)
	}
}

func TestOrderedMap_Bool(t *testing.T) {
	m := NewMap()
	m.Put("bTrue", true)
	m.Put("bFalse", false)
	m.Put("sTrue", "true")
	m.Put("sYes", "Yes")
	m.Put("sOn", "on")
	m.Put("sOne", "1")
	m.Put("sNo", "no")
	m.Put("iOne", 1)
	m.Put("iZero", 0)

	cases := map[string]bool{
		"bTrue": true, "bFalse": false,
		"sTrue": true, "sYes": true, "sOn": true, "sOne": true, "sNo": false,
		"iOne": true, "iZero": false, "missing": false,
	}
	for path, want := range cases {
		if got := m.Bool(path); got != want {
			t.Errorf("Bool(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestOrderedMap_Sort(t *testing.T) {
	m := NewMap()
	m.Put("Z", 1)
	m.Put("A", 2)
	m.Put("M", 3)

	m.Sort()

	want := []string{"A", "M", "Z"}
	got := m.Keys()
	if len(got) != len(want) {
		t.Fatalf("Sort() key count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Sort() keys[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOrderedMap_ToMap(t *testing.T) {
	child := NewMap()
	child.Put("id", 1)

	root := NewMap()
	root.Put("name", "Alice")
	root.Put("child", child)
	root.Put("list", []any{child, "plain"})

	native := root.ToMap()

	if native["name"] != "Alice" {
		t.Errorf("ToMap()[name] = %v, want Alice", native["name"])
	}

	childMap, ok := native["child"].(map[string]any)
	if !ok {
		t.Fatalf("ToMap()[child] type = %T, want map[string]any", native["child"])
	}
	if childMap["id"] != 1 {
		t.Errorf("ToMap()[child][id] = %v, want 1", childMap["id"])
	}

	list, ok := native["list"].([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("ToMap()[list] = %v, want a 2-element []any", native["list"])
	}
	if _, ok := list[0].(map[string]any); !ok {
		t.Errorf("ToMap()[list][0] type = %T, want map[string]any", list[0])
	}
	if list[1] != "plain" {
		t.Errorf("ToMap()[list][1] = %v, want plain", list[1])
	}
}

func TestOrderedMap_ToJSON(t *testing.T) {
	m := NewMap()
	m.Put("B", 2)
	m.Put("A", 1)

	got, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}
	// Insertion order (B, A) must be preserved, not alphabetical.
	want := `{"B":2,"A":1}`
	if got != want {
		t.Errorf("ToJSON() = %s, want %s", got, want)
	}
}
