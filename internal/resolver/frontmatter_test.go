package resolver

import (
	"reflect"
	"testing"
)

func TestParseSkillDeps_InlineList(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "bracket notation",
			content: `---
name: my-skill
deps: [source/skill-a, source/skill-b]
---
# My Skill
`,
			want: []string{"source/skill-a", "source/skill-b"},
		},
		{
			name: "comma separated without brackets",
			content: `---
deps: source/skill-a, source/skill-b, source/skill-c
---
`,
			want: []string{"source/skill-a", "source/skill-b", "source/skill-c"},
		},
		{
			name: "single dep no brackets",
			content: `---
deps: source/only-one
---
`,
			want: []string{"source/only-one"},
		},
		{
			name: "quoted values in brackets",
			content: `---
deps: ["source/skill-a", "source/skill-b"]
---
`,
			want: []string{"source/skill-a", "source/skill-b"},
		},
		{
			name: "single-quoted values",
			content: `---
deps: ['source/skill-a', 'source/skill-b']
---
`,
			want: []string{"source/skill-a", "source/skill-b"},
		},
		{
			name: "extra whitespace around entries",
			content: `---
deps: [  source/skill-a ,  source/skill-b  ]
---
`,
			want: []string{"source/skill-a", "source/skill-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSkillDeps(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSkillDeps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSkillDeps_NoDeps(t *testing.T) {
	content := `---
name: my-skill
version: 1.0.0
---
# My Skill
Some content here.
`
	got := ParseSkillDeps(content)
	if got != nil {
		t.Errorf("expected nil when no deps field, got %v", got)
	}
}

func TestParseSkillDeps_NoFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "no frontmatter at all",
			content: "# My Skill\nSome content.",
		},
		{
			name:    "empty content",
			content: "",
		},
		{
			name:    "frontmatter not at top",
			content: "some text\n---\ndeps: [a, b]\n---\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSkillDeps(tt.content)
			if got != nil {
				t.Errorf("expected nil for content without valid frontmatter, got %v", got)
			}
		})
	}
}

func TestParseSkillDeps_EmptyDeps(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "empty bracket list",
			content: `---
deps: []
---
`,
		},
		{
			name: "deps key with empty value",
			content: `---
deps:
---
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSkillDeps(tt.content)
			// An empty deps declaration should produce nil (no deps).
			if len(got) != 0 {
				t.Errorf("expected empty/nil deps, got %v", got)
			}
		})
	}
}

func TestParseSkillDeps_MultilineBlockList(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "basic block list",
			content: `---
name: my-skill
deps:
  - source/skill-a
  - source/skill-b
---
# My Skill
`,
			want: []string{"source/skill-a", "source/skill-b"},
		},
		{
			name: "block list with quotes",
			content: `---
deps:
  - "source/skill-a"
  - 'source/skill-b'
---
`,
			want: []string{"source/skill-a", "source/skill-b"},
		},
		{
			name: "block list after other fields",
			content: `---
name: test
version: 1.0.0
deps:
  - source/base
description: after deps
---
`,
			want: []string{"source/base"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSkillDeps(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSkillDeps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSkillDeps_DepsAfterOtherFields(t *testing.T) {
	content := `---
name: complex-skill
version: 2.0.0
author: someone
deps: [source/base, source/util]
description: a skill with deps
---
# Complex Skill
`
	got := ParseSkillDeps(content)
	want := []string{"source/base", "source/util"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseSkillDeps() = %v, want %v", got, want)
	}
}
