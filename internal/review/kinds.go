// Package review runs external, read-only agent reviews against a project
// and stores the reports and parsed findings under the xplan home.
package review

import "github.com/ffqa/xpier-xplan/internal/i18n"

// Kind describes one job type and the instructions given to the agent.
// Review kinds end with a verdict and findings; the others (ask, intent)
// produce free-form Markdown.
type Kind struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Review       bool   `json:"review"`
	Instructions string `json:"-"`
}

// Kind IDs with special handling.
const (
	KindAsk    = "ask"
	KindIntent = "intent"
)

// Kinds is the ordered list of review kinds shown in the UI.
var Kinds = []Kind{
	{
		ID:          "drift",
		Title:       "Drift check",
		Description: "Is recent work still serving the original intent? Detects scope creep and ideas leaking in from other projects.",
		Review:      true,
		Instructions: `Judge alignment with the intent. Compare the Intent and Decisions above against what the project has actually been doing recently (commits, changed docs, xteam slices, current code). Answer:
1. Is recent work moving toward the stated goal?
2. Scope creep: work that serves none of the goals, or contradicts a non-goal.
3. Contamination: concepts, naming, dependencies or features that look like they belong to a different project rather than this one.
4. Decisions that were made in code but never written down (propose ADR titles).
Verdict mapping: ok = aligned, attention = drifting, critical = off-track.
If the intent is empty, say so, infer the de-facto intent from README/specs, and use verdict attention.
If the intent is marked as an unconfirmed draft, still compare against it, but say which drift findings depend on draft statements the owner may correct.`,
	},
	{
		ID:           "code",
		Title:        "Code review",
		Description:  "Architecture, maintainability, error handling and hotspots.",
		Review:       true,
		Instructions: `Assess code quality and architecture: structure versus the stated intent, module boundaries, duplication, error handling (are failures handled or silently ignored?), naming, complexity hotspots, dead code, and consistency with the project's own conventions (AGENTS.md, README, contributing docs). Point to concrete files. Prefer the 5-10 most important findings over an exhaustive list.`,
	},
	{
		ID:           "security",
		Title:        "Security review",
		Description:  "Secrets, injection, auth gaps, insecure defaults, risky dependencies.",
		Review:       true,
		Instructions: `Look for security issues: secrets or credentials committed to the repository, injection (SQL, shell command, template), missing authentication or authorization checks, unsafe deserialization, path traversal, SSRF, weak crypto, insecure defaults (debug on, permissive CORS), dependency risks visible from manifests and lockfiles, and sensitive data written to logs. Never print secret values; reference file and line only.`,
	},
	{
		ID:           "deploy",
		Title:        "Deploy review",
		Description:  "Build, release and operations readiness.",
		Review:       true,
		Instructions: `Assess deployability and operations: how the project is built and shipped (scripts, Dockerfiles, CI workflows, Makefiles, release docs), environment configuration and secret handling, reproducibility of builds, health checks, rollback path, versioning, and whether the deploy documentation matches the actual scripts. Flag anything that would break a fresh deploy on a new machine.`,
	},
	{
		ID:           "test",
		Title:        "Test review",
		Description:  "What is covered, what is not, and whether the tests can fail.",
		Review:       true,
		Instructions: `Assess testing: which tests exist and what they cover, critical paths without tests (map them to features in the intent and specs), test quality (do assertions actually fail when behaviour breaks?), how tests are run (commands, CI gating), and flaky or environment-dependent tests. Do not run the tests; infer from the files.`,
	},
	{
		ID:           KindAsk,
		Title:        "Question",
		Description:  "Ask a read-only agent anything about the project.",
		Instructions: `Answer the owner's question about this project. Be direct, cite files, and say what you checked when the answer is uncertain.`,
	},
	{
		ID:          KindIntent,
		Title:       "Intent draft",
		Description: "Reconstruct why an existing project was started, from its README, docs and git history, for you to correct.",
		Instructions: `The owner never wrote down why this project exists. Reconstruct it from evidence so the owner can correct it: README and other docs, the earliest commits listed above, specs, changelog, and the shape of the code.
Rules:
- Ground every statement in evidence. Mark anything you inferred rather than read with "(inferred)". Never invent goals the evidence does not support.
- The original motivation (earliest docs and commits) matters more than what the project has become. When the two differ, say so under Constraints.
- Short bullet points from the owner's perspective. No marketing language.`,
	},
}

// LocalizedKinds returns Kinds with titles and descriptions in the UI
// language; Instructions (sent to agents) stay in English.
func LocalizedKinds() []Kind {
	localized := make([]Kind, len(Kinds))
	for index, kind := range Kinds {
		kind.Title = i18n.T(kind.Title)
		kind.Description = i18n.T(kind.Description)
		localized[index] = kind
	}
	return localized
}

// KindTitle returns the localized title of a kind, or its ID.
func KindTitle(id string) string {
	if kind, ok := KindByID(id); ok {
		return i18n.T(kind.Title)
	}
	return id
}

// KindByID finds a kind definition.
func KindByID(id string) (Kind, bool) {
	for _, kind := range Kinds {
		if kind.ID == id {
			return kind, true
		}
	}
	return Kind{}, false
}

// ReviewKindIDs are the kinds that produce a verdict.
func ReviewKindIDs() []string {
	ids := []string{}
	for _, kind := range Kinds {
		if kind.Review {
			ids = append(ids, kind.ID)
		}
	}
	return ids
}

// IsReviewKind reports whether jobs of this kind produce a verdict.
func IsReviewKind(id string) bool {
	kind, ok := KindByID(id)
	return ok && kind.Review
}
