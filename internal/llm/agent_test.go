package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bprendie/omasub/internal/config"
)

func TestResolveConfigUsesCurrentOmarchyAgent(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, bin, "omarchy-default-agent", "#!/bin/sh\nprintf 'crush\\n'\n")
	writeExecutable(t, bin, "crush", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)

	got, err := ResolveConfig(context.Background(), config.LLMConfig{Provider: "omarchy"})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if got.Model != "crush" {
		t.Fatalf("model = %q", got.Model)
	}
}

func TestResolveConfigExplainsMissingDefault(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, bin, "omarchy-default-agent", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	_, err := ResolveConfig(context.Background(), config.LLMConfig{Provider: "omarchy"})
	if err == nil || !strings.Contains(err.Error(), "omarchy default agent") {
		t.Fatalf("error = %v", err)
	}
}

func TestOmarchyCompletionUsesIsolatedDirectory(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, bin, "crush", "#!/bin/sh\nprintf '{\"track_ids\":[\"a\"]}'\n")
	t.Setenv("PATH", bin)
	output, err := completeWithAgent(context.Background(), "crush", []Message{{Role: "user", Content: "choose"}}, 100)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if output != `{"track_ids":["a"]}` {
		t.Fatalf("output = %q", output)
	}
}

func TestAgentPromptForbidsToolsAndPreservesRoles(t *testing.T) {
	prompt := agentPrompt([]Message{{Role: "system", Content: "rules"}, {Role: "user", Content: "request"}}, 250)
	for _, required := range []string{"Do not use tools", "<SYSTEM>", "rules", "<USER>", "request", "250 tokens"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt missing %q: %s", required, prompt)
		}
	}
}

func TestAgentCommandCoversOmarchyDispatcher(t *testing.T) {
	for _, agent := range []string{"crush", "codex", "claude", "opencode", "gemini", "copilot", "omp", "pi", "grok"} {
		name, args, err := agentCommand(agent, "prompt")
		if err != nil || name != agent || len(args) == 0 {
			t.Fatalf("agent=%s name=%s args=%v err=%v", agent, name, args, err)
		}
	}
}

func writeExecutable(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
