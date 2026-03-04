package config

import "testing"

func TestFindBundle(t *testing.T) {
	m := &ProjectManifest{
		Bundles: []BundleEntry{
			{Name: "web", Skills: []string{"a/b", "c/d"}},
		},
	}
	b, ok := FindBundle(m, "web")
	if !ok {
		t.Fatal("expected to find bundle")
	}
	if len(b.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(b.Skills))
	}
	if b.Skills[0] != "a/b" || b.Skills[1] != "c/d" {
		t.Errorf("expected skills [a/b, c/d], got %v", b.Skills)
	}
}

func TestFindBundle_NotFound(t *testing.T) {
	m := &ProjectManifest{}
	_, ok := FindBundle(m, "missing")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestFindBundle_NilManifest(t *testing.T) {
	_, ok := FindBundle(nil, "any")
	if ok {
		t.Fatal("expected not found for nil manifest")
	}
}

func TestUpsertBundle_New(t *testing.T) {
	m := &ProjectManifest{}
	UpsertBundle(m, BundleEntry{Name: "test", Skills: []string{"a/b"}})
	if len(m.Bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(m.Bundles))
	}
	if m.Bundles[0].Name != "test" {
		t.Errorf("expected name 'test', got %q", m.Bundles[0].Name)
	}
}

func TestUpsertBundle_Update(t *testing.T) {
	m := &ProjectManifest{
		Bundles: []BundleEntry{
			{Name: "test", Skills: []string{"a/b"}},
		},
	}
	UpsertBundle(m, BundleEntry{Name: "test", Skills: []string{"c/d", "e/f"}})
	if len(m.Bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(m.Bundles))
	}
	if len(m.Bundles[0].Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(m.Bundles[0].Skills))
	}
	if m.Bundles[0].Skills[0] != "c/d" || m.Bundles[0].Skills[1] != "e/f" {
		t.Errorf("expected skills [c/d, e/f], got %v", m.Bundles[0].Skills)
	}
}

func TestRemoveBundle(t *testing.T) {
	m := &ProjectManifest{
		Bundles: []BundleEntry{
			{Name: "a", Skills: []string{"x/y"}},
			{Name: "b", Skills: []string{"z/w"}},
		},
	}
	ok := RemoveBundle(m, "a")
	if !ok {
		t.Fatal("expected removal")
	}
	if len(m.Bundles) != 1 {
		t.Fatalf("expected 1 bundle remaining, got %d", len(m.Bundles))
	}
	if m.Bundles[0].Name != "b" {
		t.Error("wrong bundle remaining")
	}
}

func TestRemoveBundle_NotFound(t *testing.T) {
	m := &ProjectManifest{}
	ok := RemoveBundle(m, "missing")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestRemoveBundle_NilManifest(t *testing.T) {
	ok := RemoveBundle(nil, "any")
	if ok {
		t.Fatal("expected not found for nil manifest")
	}
}

func TestUpsertBundle_NilManifest(t *testing.T) {
	// Should not panic
	UpsertBundle(nil, BundleEntry{Name: "test", Skills: []string{"a/b"}})
}
