package scene

import "testing"

func TestCatalogContainsMVPCategories(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, def := range catalog.All() {
		counts[def.Category]++
		if len(def.Hints) < 3 {
			t.Errorf("scene %s has only %d hints", def.ID, len(def.Hints))
		}
	}
	minimums := map[string]int{"linux": 4, "git": 3, "process": 2, "http": 2}
	for category, minimum := range minimums {
		if counts[category] < minimum {
			t.Errorf("category %s has %d scenes; want at least %d", category, counts[category], minimum)
		}
	}
	if len(catalog.All()) < 11 {
		t.Fatalf("catalog has %d scenes; want at least 11", len(catalog.All()))
	}
}

func TestCatalogIDsAreUnique(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, def := range catalog.All() {
		if seen[def.ID] {
			t.Fatalf("duplicate scene ID %q", def.ID)
		}
		seen[def.ID] = true
	}
}
