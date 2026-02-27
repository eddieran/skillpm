package observation

import (
	"testing"
)

func TestNewSkillIndex(t *testing.T) {
	refs := []string{"source/code-review", "clawhub/go-test-helper"}
	idx := NewSkillIndex(refs)

	if got := idx.Resolve("code-review"); got != "source/code-review" {
		t.Errorf("Resolve(code-review) = %q, want source/code-review", got)
	}
	if got := idx.Resolve("go-test-helper"); got != "clawhub/go-test-helper" {
		t.Errorf("Resolve(go-test-helper) = %q, want clawhub/go-test-helper", got)
	}
}

func TestSkillIndex_ResolveUnknown(t *testing.T) {
	idx := NewSkillIndex([]string{"source/skill-a"})
	if got := idx.Resolve("unknown"); got != "unknown" {
		t.Errorf("Resolve(unknown) = %q, want unknown (fallback)", got)
	}
}

func TestSkillIndex_NilSafe(t *testing.T) {
	var idx *SkillIndex
	if got := idx.Resolve("anything"); got != "anything" {
		t.Errorf("nil Resolve = %q, want anything", got)
	}
	if names := idx.KnownDirNames(); names != nil {
		t.Errorf("nil KnownDirNames = %v, want nil", names)
	}
}

func TestSkillIndex_KnownDirNames(t *testing.T) {
	refs := []string{"s/a", "s/b", "s/c"}
	idx := NewSkillIndex(refs)
	known := idx.KnownDirNames()
	if len(known) != 3 {
		t.Fatalf("KnownDirNames len = %d, want 3", len(known))
	}
	for _, name := range []string{"a", "b", "c"} {
		if !known[name] {
			t.Errorf("KnownDirNames missing %q", name)
		}
	}
}

func TestSkillIndex_NestedRef(t *testing.T) {
	refs := []string{"source/path/nested/skill-name"}
	idx := NewSkillIndex(refs)
	if got := idx.Resolve("skill-name"); got != "source/path/nested/skill-name" {
		t.Errorf("Resolve(skill-name) = %q, want source/path/nested/skill-name", got)
	}
}

func TestSkillIndex_DuplicateDirName(t *testing.T) {
	// First one wins
	refs := []string{"source1/skill-a", "source2/skill-a"}
	idx := NewSkillIndex(refs)
	if got := idx.Resolve("skill-a"); got != "source1/skill-a" {
		t.Errorf("Resolve with dup = %q, want source1/skill-a (first wins)", got)
	}
}

func TestExtractDirName(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"source/skill", "skill"},
		{"source/path/skill", "skill"},
		{"skill", "skill"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := extractDirName(tt.ref); got != tt.want {
			t.Errorf("extractDirName(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}
