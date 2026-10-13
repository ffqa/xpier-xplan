package config

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ffqa/xpier-xplan/internal/i18n"
)

// Placeholders a reviewer command may contain. {prompt} is substituted last
// so prompt text can never inject another placeholder.
const (
	SlotPrompt     = "{prompt}"
	SlotPromptFile = "{prompt_file}"
	SlotPath       = "{path}"
	SlotHome       = "{xplan_home}"
	// SlotModel and SlotAgent are optional: an argument holding one of them
	// is dropped while the value is empty, so "--model={model}" disappears
	// and the agent CLI falls back to its own default.
	SlotModel = "{model}"
	SlotAgent = "{agent}"
)

// ReviewerDef describes how to invoke one agent CLI in a read-only mode.
// Command is an argv template (see the Slot constants). No shell is involved.
type ReviewerDef struct {
	Description string   `yaml:"description,omitempty" json:"description"`
	Command     []string `yaml:"command,omitempty" json:"command"`
	Model       string   `yaml:"model,omitempty" json:"model"`
	Agent       string   `yaml:"agent,omitempty" json:"agent"`
	// Timeout overrides the global review_timeout for this reviewer, e.g. "40m".
	Timeout string `yaml:"timeout,omitempty" json:"timeout"`
}

// Supports reports whether the command has a place for slot ({model}/{agent}).
func (d ReviewerDef) Supports(slot string) bool {
	return slices.ContainsFunc(d.Command, func(part string) bool { return strings.Contains(part, slot) })
}

func (d ReviewerDef) isZero() bool {
	return d.Description == "" && len(d.Command) == 0 && d.Model == "" && d.Agent == "" && d.Timeout == ""
}

// BuiltinReviewers returns a fresh copy of the presets xplan ships. Each one
// uses the agent's own read-only mode, verified by asking it to write a file.
func BuiltinReviewers() map[string]ReviewerDef {
	return map[string]ReviewerDef{
		"cursor-agent": {
			Description: "Cursor Agent in ask mode (read-only Q&A)",
			Command:     []string{"cursor-agent", "-p", "--output-format", "text", "--mode", "ask", "--trust", "--workspace", SlotPath, "--model=" + SlotModel, SlotPrompt},
		},
		"claude": {
			Description: "Claude Code print mode limited to Read/Grep/Glob, without MCP servers",
			// The tool flags are variadic: "=" keeps them from swallowing the prompt.
			// --tools is the hard allowlist; --allowedTools alone still lets
			// tools the user's own settings allow (Bash, MCP writers) through.
			Command: []string{"claude", "-p", SlotPrompt, "--tools=Read,Grep,Glob", "--allowedTools=Read,Grep,Glob", "--disallowedTools=Edit,Write,Bash,NotebookEdit", "--strict-mcp-config", "--model=" + SlotModel, "--agent=" + SlotAgent},
		},
		"codex": {
			Description: "Codex exec in read-only sandbox",
			Command:     []string{"codex", "exec", "--sandbox", "read-only", "--skip-git-repo-check", "-C", SlotPath, "--model=" + SlotModel, SlotPrompt},
		},
		"omp": {
			Description: "omp print mode with read-only tools; skills, extensions, rules and LSP stay off so it starts fast",
			Command:     []string{"omp", "-p", "--no-session", "--no-skills", "--no-extensions", "--no-rules", "--no-lsp", "--no-title", "--cwd", SlotPath, "--tools", "read,grep,glob", "--model=" + SlotModel, SlotPrompt},
		},
		"kimi": {
			// The agent stays fixed: the profile is what keeps kimi read-only.
			Description: "Kimi Code prompt mode with a Read/Grep/Glob-only agent profile",
			Command:     []string{"kimi", "-p", SlotPrompt, "--agent-file", SlotHome + "/" + KimiAgentFile, "--model=" + SlotModel},
		},
		"gemini": {
			Description: "Gemini CLI headless mode; approval-required tools are denied",
			Command:     []string{"gemini", "-p", SlotPrompt, "--approval-mode", "default", "--model=" + SlotModel},
		},
		"qwen": {
			Description: "Qwen Code headless mode; approval-required tools are denied",
			Command:     []string{"qwen", "-p", SlotPrompt, "--approval-mode", "default", "--model=" + SlotModel},
		},
		"opencode": {
			Description: "OpenCode run with the read-only plan agent",
			Command:     []string{"opencode", "run", "--agent=" + SlotAgent, "--model=" + SlotModel, SlotPrompt},
			Agent:       "plan",
		},
		"kiro": {
			Description: "Kiro CLI non-interactive chat trusting only fs_read",
			Command:     []string{"kiro-cli", "chat", "--no-interactive", "--trust-tools=fs_read", "--agent=" + SlotAgent, "--model=" + SlotModel, SlotPrompt},
		},
		"devin": {
			Description: "Devin CLI print mode; tools that need approval are rejected",
			Command:     []string{"devin", "-p", SlotPrompt, "--permission-mode", "auto", "--respect-workspace-trust", "false", "--model=" + SlotModel},
		},
	}
}

// ReviewerTemplate is a starting point for a custom reviewer: a third-party
// CLI whose read-only flags are known but that xplan could not verify end to
// end, so it is never added (or picked by "auto") without the user's say.
type ReviewerTemplate struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Command     []string `json:"command"`
	Note        string   `json:"note,omitempty"`
}

// ReviewerTemplates are offered by "Add reviewer" in Settings.
var ReviewerTemplates = []ReviewerTemplate{
	{
		Name:        "codebuddy",
		Description: "Tencent CodeBuddy Code print mode limited to Read/Grep/Glob",
		Command:     []string{"codebuddy", "-p", SlotPrompt, "--allowedTools=Read,Grep,Glob", "--disallowedTools=Edit,Write,MultiEdit,Bash,NotebookEdit", "--model=" + SlotModel},
		Note:        "Log in once by running codebuddy in a terminal.",
	},
	{
		Name:        "qodercli",
		Description: "Qoder CLI print mode with edit and shell tools disallowed",
		Command:     []string{"qodercli", "-q", "--allowed-tools=Read,Grep,Glob", "--disallowed-tools=Bash,Edit,Write,MultiEdit", "-p", SlotPrompt},
		Note:        "Log in once by running qodercli and typing /login.",
	},
	{
		Name:        "claude-provider",
		Description: "Claude Code read-only against an Anthropic-compatible provider (GLM, Kimi, DeepSeek, a relay…)",
		Command:     []string{"claude", "-p", SlotPrompt, "--settings=" + SlotHome + "/agents/claude-provider.json", "--tools=Read,Grep,Glob", "--allowedTools=Read,Grep,Glob", "--disallowedTools=Edit,Write,Bash,NotebookEdit", "--strict-mcp-config", "--model=" + SlotModel},
		Note:        `Create the settings file first, e.g. {"env": {"ANTHROPIC_BASE_URL": "https://…", "ANTHROPIC_AUTH_TOKEN": "…"}}. The key stays in that file, not in config.yaml.`,
	},
}

// previousPresets lists built-in commands that earlier versions wrote into
// config.yaml. A stored command that matches one is not a user change, so
// the current preset replaces it (along with the stored description).
var previousPresets = map[string][][]string{
	"cursor-agent": {
		{"cursor-agent", "-p", "--output-format", "text", "--mode", "ask", "--trust", "--workspace", "{path}", "{prompt}"},
	},
	"claude": {
		// --disallowedTools is variadic and swallowed the prompt.
		{"claude", "-p", "--allowedTools", "Read,Grep,Glob", "--disallowedTools", "Edit,Write,Bash,NotebookEdit", "{prompt}"},
		// No hard tool allowlist and the user's MCP servers stayed loaded.
		{"claude", "-p", "{prompt}", "--allowedTools=Read,Grep,Glob", "--disallowedTools=Edit,Write,Bash,NotebookEdit"},
		{"claude", "-p", "{prompt}", "--tools=Read,Grep,Glob", "--allowedTools=Read,Grep,Glob", "--disallowedTools=Edit,Write,Bash,NotebookEdit", "--strict-mcp-config"},
	},
	"codex": {
		{"codex", "exec", "--sandbox", "read-only", "--skip-git-repo-check", "-C", "{path}", "{prompt}"},
	},
	"omp": {
		// omp has no "ls" tool and rejects the whole list.
		{"omp", "-p", "--no-session", "--cwd", "{path}", "--tools", "read,grep,find,ls", "{prompt}"},
		{"omp", "-p", "--no-session", "--cwd", "{path}", "--tools", "read,grep,glob", "{prompt}"},
	},
	"kimi":     {{"kimi", "-p", "{prompt}", "--agent-file", "{xplan_home}/" + KimiAgentFile}},
	"gemini":   {{"gemini", "-p", "{prompt}", "--approval-mode", "default"}},
	"qwen":     {{"qwen", "-p", "{prompt}", "--approval-mode", "default"}},
	"opencode": {{"opencode", "run", "--agent", "plan", "{prompt}"}},
	"kiro":     {{"kiro-cli", "chat", "--no-interactive", "--trust-tools=fs_read", "{prompt}"}},
}

func isPreviousPreset(name string, command []string) bool {
	return slices.ContainsFunc(previousPresets[name], func(previous []string) bool {
		return slices.Equal(previous, command)
	})
}

// IsBuiltinReviewer reports whether name is one of the shipped presets.
func IsBuiltinReviewer(name string) bool {
	_, ok := BuiltinReviewers()[name]
	return ok
}

// IsModifiedReviewer reports whether a built-in preset differs from what
// xplan ships (any stored override).
func IsModifiedReviewer(name string, def ReviewerDef) bool {
	preset, ok := BuiltinReviewers()[name]
	if !ok {
		return false
	}
	return !overrideOf(preset, def).isZero()
}

// overlayReviewer applies the non-empty stored fields onto a preset.
func overlayReviewer(preset, stored ReviewerDef, name string) ReviewerDef {
	if isPreviousPreset(name, stored.Command) {
		stored.Command = nil
		stored.Description = ""
	}
	merged := preset
	if stored.Description != "" {
		merged.Description = stored.Description
	}
	if len(stored.Command) > 0 {
		merged.Command = stored.Command
	}
	if stored.Model != "" {
		merged.Model = stored.Model
	}
	if stored.Agent != "" {
		merged.Agent = stored.Agent
	}
	if stored.Timeout != "" {
		merged.Timeout = stored.Timeout
	}
	return merged
}

// overrideOf is the part of def that differs from preset. Empty model and
// agent mean "the preset's default", so clearing a field resets it.
func overrideOf(preset, def ReviewerDef) ReviewerDef {
	override := ReviewerDef{Model: def.Model, Agent: def.Agent, Timeout: def.Timeout}
	if def.Description != preset.Description {
		override.Description = def.Description
	}
	if !slices.Equal(def.Command, preset.Command) {
		override.Command = def.Command
	}
	if override.Model == preset.Model {
		override.Model = ""
	}
	if override.Agent == preset.Agent {
		override.Agent = ""
	}
	if override.Timeout == preset.Timeout {
		override.Timeout = ""
	}
	return override
}

// storedReviewers is what goes into config.yaml: custom reviewers in full,
// built-in presets only as overrides, so preset fixes in new versions reach
// existing installs.
func storedReviewers(reviewers map[string]ReviewerDef) map[string]ReviewerDef {
	builtins := BuiltinReviewers()
	stored := map[string]ReviewerDef{}
	for name, def := range reviewers {
		preset, isBuiltin := builtins[name]
		if !isBuiltin {
			stored[name] = def
			continue
		}
		if override := overrideOf(preset, def); !override.isZero() {
			stored[name] = override
		}
	}
	return stored
}

var reviewerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,39}$`)

// ErrReviewer is matched by every ValidateReviewer error.
var ErrReviewer = errors.New("invalid reviewer")

// reviewerError is a localized validation message that matches ErrReviewer.
type reviewerError string

func (e reviewerError) Error() string        { return string(e) }
func (e reviewerError) Is(target error) bool { return target == ErrReviewer }

// ValidateReviewer checks a reviewer definition before it is saved.
func ValidateReviewer(name string, def ReviewerDef) error {
	if name == AutoReviewer || !reviewerNamePattern.MatchString(name) {
		return reviewerError(i18n.T("Reviewer names use 1-40 lowercase letters, digits, dots, dashes or underscores, and cannot be %q.", AutoReviewer))
	}
	if len(def.Command) == 0 || strings.TrimSpace(def.Command[0]) == "" {
		return reviewerError(i18n.T("The command needs at least the program to run."))
	}
	if strings.Contains(def.Command[0], "{") {
		return reviewerError(i18n.T("The program (first line of the command) cannot be a placeholder."))
	}
	if !def.Supports(SlotPrompt) && !def.Supports(SlotPromptFile) {
		return reviewerError(i18n.T("The command must pass the review prompt with %s or %s.", SlotPrompt, SlotPromptFile))
	}
	for _, part := range def.Command {
		if strings.ContainsAny(part, "\x00\n\r") {
			return reviewerError(i18n.T("Command arguments cannot contain line breaks."))
		}
	}
	for label, value := range map[string]string{"model": def.Model, "agent": def.Agent} {
		if err := validateSlotValue(label, value); err != nil {
			return err
		}
	}
	if def.Timeout != "" {
		parsed, err := time.ParseDuration(def.Timeout)
		if err != nil || parsed < time.Minute || parsed > 24*time.Hour {
			return reviewerError(i18n.T("The timeout must look like 30m or 1h, between 1m and 24h."))
		}
	}
	if len([]rune(def.Description)) > 300 {
		return reviewerError(i18n.T("The description is longer than 300 characters."))
	}
	return nil
}

func validateSlotValue(label, value string) error {
	switch {
	case value == "":
		return nil
	case len(value) > 200:
		return reviewerError(i18n.T("The %s is longer than 200 characters.", label))
	case strings.HasPrefix(value, "-"):
		return reviewerError(i18n.T("The %s cannot start with \"-\".", label))
	case strings.ContainsAny(value, "{}\x00\n\r\t "):
		return reviewerError(i18n.T("The %s cannot contain spaces, braces or line breaks.", label))
	}
	return nil
}
