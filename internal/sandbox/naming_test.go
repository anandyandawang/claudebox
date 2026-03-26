package sandbox

import (
	"regexp"
	"strings"
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

	// Format: <wshash(2)>-<workspace(12)>.<MMDD>-<cat(5)>-<hash(2)>
	pattern := `^[0-9a-f]{2}-[a-zA-Z0-9_.-]{1,12}\.\d{4}-[a-z]{1,5}-[0-9a-f]{2}$`
	if matched, _ := regexp.MatchString(pattern, name); !matched {
		t.Errorf("GenerateSandboxName = %q, want match %s", name, pattern)
	}

	// Max 29 chars
	if len(name) > 29 {
		t.Errorf("GenerateSandboxName length = %d, want <= 29", len(name))
	}
}

func TestGenerateSandboxNameTruncatesLongWorkspace(t *testing.T) {
	name := GenerateSandboxName("/path/to/lambda-jpm-clearings", "jvm")

	// "lambda-jpm-clearings" is 20 chars, should truncate to 12
	// Workspace part is between first dash and the dot
	dotIdx := strings.Index(name, ".")
	if dotIdx == -1 {
		t.Fatalf("no dot in name: %q", name)
	}
	wsPart := name[:dotIdx] // e.g. "b4-lambda-jpm-c"
	// wshash is 2 chars + dash = 3, so workspace is wsPart[3:]
	ws := wsPart[3:]
	if len(ws) > 12 {
		t.Errorf("workspace portion %q exceeds 12 chars", ws)
	}
}

func TestGenerateSandboxNameTruncatesLongTemplate(t *testing.T) {
	name := GenerateSandboxName("/path/to/myapp", "kotlin-spring")

	// Template feeds into the hash but doesn't appear in the name directly
	// Just verify the name is valid and within length
	if len(name) > 29 {
		t.Errorf("name length = %d, want <= 29", len(name))
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

func TestGenerateSandboxNameMaxLength(t *testing.T) {
	// Worst case: 12-char workspace, 5-char cat name
	// Run many times to exercise different cat names
	for i := 0; i < 100; i++ {
		name := GenerateSandboxName("/path/to/abcdefghijklmnopqrstuvwxyz", "long-template-name")
		if len(name) > 29 {
			t.Errorf("iteration %d: name %q length = %d, want <= 29", i, name, len(name))
		}
	}
}

func TestGenerateSandboxNameSocketPathFits(t *testing.T) {
	// Simulate a long home directory: /Users/christopherjohnson (25 chars)
	homeDir := "/Users/christopherjohnson"
	name := GenerateSandboxName("/path/to/some-long-workspace-name", "jvm")

	socketPath := homeDir + "/.docker/sandboxes/vm/" + name + "/docker-public.sock"
	if len(socketPath) > 103 {
		t.Errorf("socket path length = %d, want <= 103: %s", len(socketPath), socketPath)
	}
}

func TestWorkspacePrefix(t *testing.T) {
	prefix := WorkspacePrefix("/path/to/lambda-jpm-clearings")

	// Should end with a dot
	if !strings.HasSuffix(prefix, ".") {
		t.Errorf("WorkspacePrefix = %q, want suffix '.'", prefix)
	}

	// Should match the beginning of a generated name for the same workspace
	name := GenerateSandboxName("/path/to/lambda-jpm-clearings", "jvm")
	if !strings.HasPrefix(name, prefix) {
		t.Errorf("name %q does not start with prefix %q", name, prefix)
	}

	// Different workspace with same 12-char prefix should get a different prefix
	prefix2 := WorkspacePrefix("/path/to/lambda-jpm-clients")
	if prefix == prefix2 {
		t.Errorf("different workspaces got same prefix: %q", prefix)
	}
}
