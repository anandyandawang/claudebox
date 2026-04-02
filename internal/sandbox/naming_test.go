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
	id := GenerateSandboxID("jvm")
	name := GenerateSandboxName("/path/to/my-project", id)

	// Format: <wshash(2)>-<workspace(12)>.<MMDD>-<cat(5)>-<hash(2)>
	pattern := `^[0-9a-f]{2}-[a-zA-Z0-9_.-]{1,12}\.\d{4}-[a-z]{5}-[0-9a-f]{2}$`
	if matched, _ := regexp.MatchString(pattern, name); !matched {
		t.Errorf("GenerateSandboxName = %q, want match %s", name, pattern)
	}

	// Max length
	if len(name) > maxSandboxNameLen {
		t.Errorf("GenerateSandboxName length = %d, want <= %d", len(name), maxSandboxNameLen)
	}
}

func TestGenerateSandboxNameTruncatesLongWorkspace(t *testing.T) {
	id := GenerateSandboxID("jvm")
	name := GenerateSandboxName("/path/to/lambda-jpm-clearings", id)

	// "lambda-jpm-clearings" is 20 chars, should truncate to 12
	dotIdx := strings.Index(name, ".")
	if dotIdx == -1 {
		t.Fatalf("no dot in name: %q", name)
	}
	ws := name[3:dotIdx] // skip wshash(2) + dash
	if len(ws) > 12 {
		t.Errorf("workspace portion %q exceeds 12 chars", ws)
	}
}

func TestGenerateSandboxNameTruncatesLongTemplate(t *testing.T) {
	id := GenerateSandboxID("kotlin-spring")
	name := GenerateSandboxName("/path/to/myapp", id)

	if len(name) > maxSandboxNameLen {
		t.Errorf("name length = %d, want <= %d", len(name), maxSandboxNameLen)
	}
}

func TestDegenerateWorkspaceNames(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"all dots", "/path/to/..."},
		{"root", "/"},
		{"all special chars", "/path/to/@@@"},
		{"hyphens only", "/path/to/---"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := GenerateSandboxName(tt.path, GenerateSandboxID("jvm"))

			// Must still match the format and length constraint.
			pattern := `^[0-9a-f]{2}-[a-zA-Z0-9_.-]{1,12}\.\d{4}-[a-z]{5}-[0-9a-f]{2}$`
			if matched, _ := regexp.MatchString(pattern, name); !matched {
				t.Errorf("GenerateSandboxName(%q) = %q, want match %s", tt.path, name, pattern)
			}
			if len(name) > maxSandboxNameLen {
				t.Errorf("length = %d, want <= %d", len(name), maxSandboxNameLen)
			}

			// Prefix must work too.
			prefix := WorkspacePrefix(tt.path)
			if !strings.HasPrefix(name, prefix) {
				t.Errorf("name %q does not start with prefix %q", name, prefix)
			}
		})
	}
}

func TestGenerateSandboxID(t *testing.T) {
	id := GenerateSandboxID("jvm")
	// Format: MMDD-cat(5)-hash(2), e.g. "0401-chonk-f3"
	pattern := `^\d{4}-[a-z]{5}-[0-9a-f]{2}$`
	if matched, _ := regexp.MatchString(pattern, id); !matched {
		t.Errorf("GenerateSandboxID = %q, want match %s", id, pattern)
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
	if len(name) != 5 {
		t.Errorf("randomCatName() = %q, want exactly 5 chars", name)
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
		name := GenerateSandboxName("/path/to/abcdefghijklmnopqrstuvwxyz", GenerateSandboxID("long-template-name"))
		if len(name) > maxSandboxNameLen {
			t.Errorf("iteration %d: name %q length = %d, want <= %d", i, name, len(name), maxSandboxNameLen)
		}
	}
}

func TestGenerateSandboxNameSocketPathFits(t *testing.T) {
	// Simulate a long home directory: /Users/christopherjohnson (25 chars)
	homeDir := "/Users/christopherjohnson"
	name := GenerateSandboxName("/path/to/some-long-workspace-name", GenerateSandboxID("jvm"))

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
	name := GenerateSandboxName("/path/to/lambda-jpm-clearings", GenerateSandboxID("jvm"))
	if !strings.HasPrefix(name, prefix) {
		t.Errorf("name %q does not start with prefix %q", name, prefix)
	}

	// Different workspace with same 12-char prefix should get a different prefix
	prefix2 := WorkspacePrefix("/path/to/lambda-jpm-clients")
	if prefix == prefix2 {
		t.Errorf("different workspaces got same prefix: %q", prefix)
	}
}

func TestWorkspacePrefixStable(t *testing.T) {
	// Same workspace must always produce the same prefix.
	ws := "/home/user/projects/my-cool-app"
	first := WorkspacePrefix(ws)
	for i := 0; i < 50; i++ {
		got := WorkspacePrefix(ws)
		if got != first {
			t.Fatalf("call %d: WorkspacePrefix changed from %q to %q", i, first, got)
		}
	}
}

func TestWorkspacePrefixMatchesDifferentTemplates(t *testing.T) {
	// The prefix is workspace-only; different templates must still match.
	ws := "/repos/my-service"
	prefix := WorkspacePrefix(ws)

	nameJVM := GenerateSandboxName(ws, GenerateSandboxID("jvm"))
	nameKotlin := GenerateSandboxName(ws, GenerateSandboxID("kotlin-spring"))

	if !strings.HasPrefix(nameJVM, prefix) {
		t.Errorf("jvm name %q does not start with prefix %q", nameJVM, prefix)
	}
	if !strings.HasPrefix(nameKotlin, prefix) {
		t.Errorf("kotlin-spring name %q does not start with prefix %q", nameKotlin, prefix)
	}
}

func TestCatNamesExactLength(t *testing.T) {
	for _, name := range catNames {
		if len(name) != 5 {
			t.Errorf("cat name %q is %d chars, want exactly 5", name, len(name))
		}
	}
}

func TestWorkspacePrefixDifferentPaths(t *testing.T) {
	// Same directory name, different parent paths — must get different prefixes
	// because workspaceHash hashes the full path, not just the basename.
	prefixA := WorkspacePrefix("/work/client-a/my-service")
	prefixB := WorkspacePrefix("/work/client-b/my-service")
	if prefixA == prefixB {
		t.Errorf("same basename different paths got same prefix: %q", prefixA)
	}
}

func TestWorkspacePrefixIsolatesTruncationCollisions(t *testing.T) {
	// Two workspaces that truncate to the same 12 chars ("lambda-jpm-c")
	// must produce different prefixes thanks to the workspace hash.
	wsA := "/path/to/lambda-jpm-clearings"
	wsB := "/path/to/lambda-jpm-clients"

	prefixA := WorkspacePrefix(wsA)
	prefixB := WorkspacePrefix(wsB)

	if prefixA == prefixB {
		t.Fatalf("collision: both workspaces produced prefix %q", prefixA)
	}

	// Generate several sandbox names for each workspace.
	var namesA, namesB []string
	for i := 0; i < 10; i++ {
		namesA = append(namesA, GenerateSandboxName(wsA, GenerateSandboxID("jvm")))
		namesB = append(namesB, GenerateSandboxName(wsB, GenerateSandboxID("jvm")))
	}

	// Names from workspace A must match prefix A but NOT prefix B.
	for _, n := range namesA {
		if !strings.HasPrefix(n, prefixA) {
			t.Errorf("wsA name %q does not match prefixA %q", n, prefixA)
		}
		if strings.HasPrefix(n, prefixB) {
			t.Errorf("wsA name %q incorrectly matches prefixB %q", n, prefixB)
		}
	}

	// Names from workspace B must match prefix B but NOT prefix A.
	for _, n := range namesB {
		if !strings.HasPrefix(n, prefixB) {
			t.Errorf("wsB name %q does not match prefixB %q", n, prefixB)
		}
		if strings.HasPrefix(n, prefixA) {
			t.Errorf("wsB name %q incorrectly matches prefixA %q", n, prefixA)
		}
	}
}
