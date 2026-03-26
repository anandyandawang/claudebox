package sandbox

import (
	"regexp"
	"testing"
)

func TestSanitizeWorkspaceName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"my-project", "my-project"},
		{"my project", "my-project"},
		{"project@v2!", "project-v2-"},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		got := SanitizeWorkspaceName(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeWorkspaceName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateSandboxName(t *testing.T) {
	name := GenerateSandboxName("/path/to/my-project", "jvm")
	pattern := `^my-project-jvm-sandbox-\d{8}-\d{6}$`
	if matched, _ := regexp.MatchString(pattern, name); !matched {
		t.Errorf("GenerateSandboxName = %q, want match %s", name, pattern)
	}
}

func TestGenerateSessionID(t *testing.T) {
	id := GenerateSessionID()
	pattern := `^sandbox-\d{8}-\d{6}$`
	if matched, _ := regexp.MatchString(pattern, id); !matched {
		t.Errorf("GenerateSessionID = %q, want match %s", id, pattern)
	}
}

func TestTruncateClean(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello-world", 5, "hello"},
		{"hello-", 5, "hello"},
		{"hello-", 6, "hello"},
		{"ab-cd-ef", 5, "ab-cd"},
		{"ab---", 5, "ab"},
		{"ab...", 5, "ab"},
		{"a", 12, "a"},
		{"abcdefghijklmnop", 12, "abcdefghijkl"},
		{"abcdefghijkl-", 12, "abcdefghijkl"},
		{"abcdefghijkl.", 12, "abcdefghijkl"},
	}
	for _, tt := range tests {
		got := truncateClean(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncateClean(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestWorkspaceHash(t *testing.T) {
	// Deterministic: same input -> same output
	h1 := workspaceHash("lambda-jpm-clearings")
	h2 := workspaceHash("lambda-jpm-clearings")
	if h1 != h2 {
		t.Errorf("workspaceHash not deterministic: %q != %q", h1, h2)
	}
	// Length is always 2
	if len(h1) != 2 {
		t.Errorf("workspaceHash length = %d, want 2", len(h1))
	}
	// Different inputs -> different outputs (probabilistic but reliable for these inputs)
	h3 := workspaceHash("lambda-jpm-clients")
	if h1 == h3 {
		t.Errorf("workspaceHash collision: %q and %q both produce %q", "lambda-jpm-clearings", "lambda-jpm-clients", h1)
	}
}

func TestRandomCatName(t *testing.T) {
	name := randomCatName()
	if len(name) == 0 || len(name) > 5 {
		t.Errorf("randomCatName() = %q, want 1-5 chars", name)
	}
	found := false
	for _, c := range catNames {
		if c == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("randomCatName() = %q, not in catNames list", name)
	}
}
