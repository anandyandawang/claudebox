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
