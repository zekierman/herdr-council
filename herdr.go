package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// Agent is one coding agent pane herdr knows about.
type Agent struct {
	Name      string `json:"agent"`
	Pane      string `json:"pane_id"`
	Status    string `json:"agent_status"`
	Workspace string `json:"workspace_id"`
	Title     string `json:"terminal_title_stripped"`
}

func herdrBin() string {
	if b := os.Getenv("HERDR_BIN_PATH"); b != "" {
		return b
	}
	return "herdr"
}

// herdr runs the herdr CLI with argv (never through a shell) and returns stdout.
func herdr(args ...string) ([]byte, error) {
	cmd := exec.Command(herdrBin(), args...)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return out, fmt.Errorf("herdr %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return out, fmt.Errorf("herdr %s: %w", args[0], err)
	}
	return out, nil
}

// currentWorkspace reads the workspace from the plugin context, falling back to the pane env.
func currentWorkspace() string {
	if raw := os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"); raw != "" {
		var ctx struct {
			Workspace string `json:"workspace_id"`
		}
		if json.Unmarshal([]byte(raw), &ctx) == nil && ctx.Workspace != "" {
			return ctx.Workspace
		}
	}
	return os.Getenv("HERDR_WORKSPACE_ID")
}

// listAgents returns the agents in one workspace, sorted by name.
func listAgents(workspace string) ([]Agent, error) {
	out, err := herdr("agent", "list")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Agents []Agent `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("agent list: %w", err)
	}
	var agents []Agent
	for _, a := range resp.Result.Agents {
		if workspace == "" || a.Workspace == workspace {
			agents = append(agents, a)
		}
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name+agents[i].Pane < agents[j].Name+agents[j].Pane })
	return agents, nil
}

var promptAgent = func(pane, text string) error {
	_, err := herdr("agent", "prompt", pane, text)
	return err
}

func focusAgent(pane string) error {
	_, err := herdr("agent", "focus", pane)
	return err
}
