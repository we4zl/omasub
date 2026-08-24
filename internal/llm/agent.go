package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bprendie/omasub/internal/config"
)

func ResolveConfig(ctx context.Context, cfg config.LLMConfig) (config.LLMConfig, error) {
	cfg.NormalizeForClient()
	if cfg.Provider != "omarchy" {
		return cfg, nil
	}
	agent, err := resolveDefaultAgent(ctx)
	if err != nil {
		return cfg, err
	}
	cfg.Model = agent
	return cfg, nil
}

func resolveDefaultAgent(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, "omarchy-default-agent").Output()
	if err != nil {
		return "", fmt.Errorf("resolve Omarchy default agent: %w", err)
	}
	agent := strings.TrimSpace(string(output))
	if agent == "" {
		return "", errors.New("choose a default agent with: omarchy default agent <name>")
	}
	if _, err := exec.LookPath(agent); err != nil {
		return "", fmt.Errorf("Omarchy default agent %q is not installed", agent)
	}
	return agent, nil
}

func completeWithAgent(ctx context.Context, agent string, messages []Message, maxTokens int) (string, error) {
	if strings.TrimSpace(agent) == "" {
		return "", errors.New("Omarchy agent is not resolved")
	}
	workDir, err := os.MkdirTemp("", "omasub-agent-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workDir)
	prompt := agentPrompt(messages, maxTokens)
	name, args, err := agentCommand(agent, prompt)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("Omarchy agent %s failed: %s", agent, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("run Omarchy agent %s: %w", agent, err)
	}
	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", fmt.Errorf("Omarchy agent %s returned no content", agent)
	}
	return result, nil
}

func agentPrompt(messages []Message, maxTokens int) string {
	var prompt strings.Builder
	prompt.WriteString("Return only the requested structured response. Do not use tools, inspect files, or modify the system.\n")
	if maxTokens > 0 {
		prompt.WriteString("Keep the response within " + strconv.Itoa(maxTokens) + " tokens.\n")
	}
	for _, message := range messages {
		prompt.WriteString("\n<" + strings.ToUpper(message.Role) + ">\n")
		prompt.WriteString(message.Content)
		prompt.WriteString("\n</" + strings.ToUpper(message.Role) + ">\n")
	}
	return prompt.String()
}

func agentCommand(agent, prompt string) (string, []string, error) {
	switch agent {
	case "crush":
		return agent, []string{"run", "--quiet", prompt}, nil
	case "codex":
		return agent, []string{"exec", "--sandbox", "read-only", "--ephemeral", "--skip-git-repo-check", "--color", "never", prompt}, nil
	case "claude":
		return agent, []string{"--print", "--bare", "--output-format", "text", prompt}, nil
	case "opencode":
		return agent, []string{"run", prompt}, nil
	case "gemini":
		return agent, []string{"--yolo", "--output-format", "text", "--prompt", prompt}, nil
	case "copilot":
		return agent, []string{"--allow-all", "--prompt", prompt}, nil
	case "omp":
		return agent, []string{"--auto-approve", "--prompt", prompt}, nil
	case "pi", "grok":
		return agent, []string{"--print", prompt}, nil
	default:
		return "", nil, fmt.Errorf("unsupported Omarchy default agent %q", agent)
	}
}
