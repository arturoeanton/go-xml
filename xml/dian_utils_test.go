package xml

import "testing"

func TestCalculateCUFE(t *testing.T) {
	// Known vector: sha384("01-1002025-12-1912:00:00-05:001000.0001190.00040.001190.00900123456222222222222claveTecnicaPruebas2")
	const want = "c413d03a2de3ce4f36d8d564c327ef38c9ca9946f4b072e675c8cc15bbfbd8532fab6b835439866b758fca7954bfbe33"

	got := CalculateCUFE(
		"01-100",
		"2025-12-19",
		"12:00:00-05:00",
		"1000.00",
		"01",
		"190.00",
		"04",
		"0.00",
		"1190.00",
		"900123456",
		"222222222222",
		"claveTecnicaPruebas",
		"2",
	)

	if got != want {
		t.Fatalf("CalculateCUFE() = %s, want %s", got, want)
	}

	if len(got) != 96 {
		t.Errorf("CalculateCUFE() length = %d, want 96 hex chars (SHA-384)", len(got))
	}
}

func TestCalculateCUFE_FieldOrderMatters(t *testing.T) {
	base := CalculateCUFE("01-100", "2025-12-19", "12:00:00-05:00", "1000.00",
		"01", "190.00", "04", "0.00", "1190.00", "900123456", "222222222222", "clave", "2")

	// Swapping ValFac and ValTot must not silently produce a value that
	// collides with the correctly-ordered field; guards against accidental
	// argument reordering breaking DIAN compliance.
	swapped := CalculateCUFE("01-100", "2025-12-19", "12:00:00-05:00", "1190.00",
		"01", "190.00", "04", "0.00", "1000.00", "900123456", "222222222222", "clave", "2")

	if base == swapped {
		t.Fatal("CalculateCUFE() did not change when field values were swapped")
	}
}
