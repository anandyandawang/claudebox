// internal/commands/create.go
package commands

import (
	"claudebox/internal/credentials"
	"claudebox/internal/docker"
	"claudebox/internal/environment"
	"claudebox/internal/sandbox"
	"fmt"
	"os"
	"strings"
)

// ParseCreateArgs parses [template] [workspace] [agent_args...] from positional args.
// Convention: workspace doesn't start with "-", agent args do.
func ParseCreateArgs(args []string) (template, workspace string, agentArgs []string) {
	if len(args) == 0 {
		return
	}
	template = args[0]
	rest := args[1:]
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		workspace = rest[0]
		rest = rest[1:]
	}
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	agentArgs = rest
	return
}

// RunCreate executes the create flow: validate, build, create sandbox, setup, run.
func RunCreate(d docker.Docker, templatesDir string, args []string) error {
	template, workspace, agentArgs := ParseCreateArgs(args)
	if template == "" {
		return fmt.Errorf("template name required")
	}

	mgr := sandbox.NewManager(d, templatesDir)

	// 1. Validate template
	if err := mgr.ValidateTemplate(template); err != nil {
		return err
	}

	// 2. Build image
	fmt.Printf("Building template image: %s-sandbox...\n", template)
	imageName, err := mgr.BuildImage(template)
	if err != nil {
		return err
	}

	// 3. Generate names
	sessionID := sandbox.GenerateSessionID()
	sandboxName := sandbox.GenerateSandboxName(workspace, template)
	claudeDir := os.Getenv("HOME") + "/.claude"

	// 4. Create sandbox
	fmt.Printf("Creating sandbox: %s...\n", sandboxName)
	if err := mgr.Create(sandboxName, sandbox.CreateOpts{
		ImageName: imageName,
		Workspace: workspace,
		ClaudeDir: claudeDir,
		SessionID: sessionID,
	}); err != nil {
		return err
	}

	// 5. Setup environment
	if err := environment.Setup(d, sandboxName); err != nil {
		return err
	}

	// 6. Apply and verify network policy
	fmt.Println("Applying network policy (deny by default)...")
	applied, err := mgr.ApplyNetworkPolicy(sandboxName, template)
	if err != nil {
		return err
	}
	if applied {
		fmt.Println("Verifying network policy...")
		if err := mgr.VerifyNetworkPolicy(sandboxName); err != nil {
			return err
		}
		fmt.Println("Network policy verified.")
	} else {
		fmt.Println("No allowed-hosts.txt found, using default network policy (allow all).")
	}

	// 7. Refresh credentials
	if err := credentials.Refresh(d, sandboxName); err != nil {
		return err
	}

	// 8. Wrap claude binary
	if err := mgr.WrapClaudeBinary(sandboxName); err != nil {
		return err
	}

	// 9. Run
	fmt.Println("Starting sandbox...")
	runArgs := append([]string{"--dangerously-skip-permissions"}, agentArgs...)
	return mgr.Run(sandboxName, runArgs...)
}
