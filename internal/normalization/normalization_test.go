package normalization

import (
	"reflect"
	"testing"
)

func TestExactPackages(t *testing.T) {
	for input, want := range map[string]string{"SOT23": "SOT-23", "SOT-23": "SOT-23", "SOT23-5": "SOT-23-5", "SOT-23-5": "SOT-23-5", " SOT23 ": "SOT-23", "SOT23-6": "SOT23-6", "SOT23-5L": "SOT23-5L", "SOT23 (3mm)": "SOT23 (3mm)", "sot23": "sot23", "SOT32": "SOT32", "SOT-32": "SOT-32"} {
		t.Run(input, func(t *testing.T) {
			got := Package(input)
			if got != want || Package(got) != got {
				t.Fatalf("%q => %q, want %q, idempotent", input, got, want)
			}
		})
	}
}
func TestDictionaryAndLanguage(t *testing.T) {
	s := Rules()
	for word, want := range s.Dictionary {
		r := Preview(Fields{Name: word, Description: word}, s)
		if r.Candidate.Name != want || r.Candidate.Description != want {
			t.Fatal(r)
		}
	}
	for _, word := range []string{"MOSFET", "USB Type-C", "EEPROM", "电阻 10kΩ ±1%", "芯片 STM32F103", "TI 芯片"} {
		if NeedsReview(word, "STM32F103", "TI") {
			t.Errorf("technical token marked: %s", word)
		}
	}
	for _, word := range []string{"precision resistor", "Unknown", "Resistor-0603", "resistor", "RES 10kΩ", "电阻 precision"} {
		if !NeedsReview(word, "", "") {
			t.Errorf("unknown language accepted: %s", word)
		}
		if Preview(Fields{Name: word}, s).Candidate.Name != word {
			t.Errorf("free translation: %s", word)
		}
	}
}
func TestFormalCategoriesOnly(t *testing.T) {
	s := Rules()
	s.Categories = []Category{{ID: 1, Name: "电阻"}}
	s.Historical["categories"] = []string{"Chip Resistor"}
	for _, f := range []Fields{{CategoryName: "Chip Resistor"}, {CategoryName: "电容"}, {CategoryID: 9, CategoryName: "电阻"}} {
		r := Preview(f, s)
		if r.Candidate.CategoryID != 0 || r.Candidate.CategoryName != "" {
			t.Fatal(r)
		}
	}
	if Preview(Fields{CategoryName: "Resistors"}, s).Candidate.CategoryID != 1 {
		t.Fatal("dictionary category not resolved")
	}
	s.Categories = append(s.Categories, Category{ID: 2, Name: "电阻"})
	if Preview(Fields{CategoryName: "电阻"}, s).Candidate.CategoryID != 0 {
		t.Fatal("ambiguous category resolved")
	}
	if Preview(Fields{Manufacturer: "Texas Instruments Inc."}, s).Candidate.Manufacturer != "Texas Instruments Inc." {
		t.Fatal("fuzzy brand alias")
	}
}
func TestConflictsIndependentOfOrder(t *testing.T) {
	a := Source{Name: "a", Fields: Fields{Package: "SOT23", Model: "A1"}}
	b := Source{Name: "b", Fields: Fields{Package: "SOT-23", Model: "B2"}}
	forward := Conflicts([]Source{a, b}, Rules())
	backward := Conflicts([]Source{b, a}, Rules())
	if !reflect.DeepEqual(forward, backward) || len(forward) != 1 || forward[0].Field != "model" {
		t.Fatalf("%+v / %+v", forward, backward)
	}
}
func TestConfirmationBoundToValueAndHistoricalFields(t *testing.T) {
	f := Fields{Name: "Unknown"}
	if _, err := ReviewFields(f, Fields{}, nil, nil, true); err == nil {
		t.Fatal("missing confirmation accepted")
	}
	review, err := ReviewFields(f, Fields{}, nil, map[string]string{"name": "Unknown"}, true)
	if err != nil || len(review) != 1 || !review[0].Confirmed {
		t.Fatal(review, err)
	}
	next := Fields{Name: "Changed"}
	if _, err = ReviewFields(next, f, review, map[string]string{"name": "Unknown"}, false); err == nil {
		t.Fatal("stale confirmation accepted")
	}
	if _, err = ReviewFields(f, f, nil, nil, false); err != nil {
		t.Fatal("historical unchanged rejected", err)
	}
	confirmedLegacy, err := ReviewFields(f, f, nil, map[string]string{"name": f.Name}, false)
	if err != nil || len(confirmedLegacy) != 1 || !confirmedLegacy[0].Confirmed {
		t.Fatal("explicit legacy review not recorded", confirmedLegacy, err)
	}
	next.Name = "电阻"
	review, err = ReviewFields(next, f, review, nil, false)
	if err != nil || len(review) != 0 {
		t.Fatal("corrected review not cleared", review, err)
	}
}
