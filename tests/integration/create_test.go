//go:build integration

package integration

import (
	"regexp"
	"testing"
)

func TestImageBuilds(t *testing.T) {
	buildTemplateImage(t, "jvm")
}

func TestSandboxNameFormat(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-create-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	pattern := `^cb-create-test-jvm-sandbox-\d{8}-\d{6}$`
	if matched, _ := regexp.MatchString(pattern, name); !matched {
		t.Errorf("sandbox name %q doesn't match %s", name, pattern)
	}
}
