// Package review runs external, read-only agent reviews against a project
// and stores the reports and parsed findings under the xplan home.
package review

// Kind describes one review type and the instructions given to the agent.
type Kind struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Instructions string `json:"-"`
}

// Kinds is the ordered list of review kinds shown in the UI.
var Kinds = []Kind{
	{
		ID:          "drift",
		Title:       "Drift check",
		Description: "Is recent work still serving the original intent? Detects scope creep and ideas leaking in from other projects.",
		Instructions: `Judge alignment with the intent. Compare the Intent and Decisions above against what the project has actually been doing recently (commits, changed docs, xteam slices, current code). Answer:
1. Is recent work moving toward the stated goal?
2. Scope creep: work that serves none of the goals, or contradicts a non-goal.
3. Contamination: concepts, naming, dependencies or features that look like they belong to a different project rather than this one.
4. Decisions that were made in code but never written down (propose ADR titles).
Verdict mapping: ok = aligned, attention = drifting, critical = off-track.
If the intent is empty, say so, infer the de-facto intent from README/specs, and use verdict attention.`,
	},
	{
		ID:           "code",
		Title:        "Code review",
		Description:  "Architecture, maintainability, error handling and hotspots.",
		Instructions: `Assess code quality and architecture: structure versus the stated intent, module boundaries, duplication, error handling (are failures handled or silently ignored?), naming, complexity hotspots, dead code, and consistency with the project's own conventions (AGENTS.md, README, contributing docs). Point to concrete files. Prefer the 5-10 most important findings over an exhaustive list.`,
	},
	{
		ID:           "security",
		Title:        "Security review",
		Description:  "Secrets, injection, auth gaps, insecure defaults, risky dependencies.",
		Instructions: `Look for security issues: secrets or credentials committed to the repository, injection (SQL, shell command, template), missing authentication or authorization checks, unsafe deserialization, path traversal, SSRF, weak crypto, insecure defaults (debug on, permissive CORS), dependency risks visible from manifests and lockfiles, and sensitive data written to logs. Never print secret values; reference file and line only.`,
	},
	{
		ID:           "deploy",
		Title:        "Deploy review",
		Description:  "Build, release and operations readiness.",
		Instructions: `Assess deployability and operations: how the project is built and shipped (scripts, Dockerfiles, CI workflows, Makefiles, release docs), environment configuration and secret handling, reproducibility of builds, health checks, rollback path, versioning, and whether the deploy documentation matches the actual scripts. Flag anything that would break a fresh deploy on a new machine.`,
	},
	{
		ID:           "test",
		Title:        "Test review",
		Description:  "What is covered, what is not, and whether the tests can fail.",
		Instructions: `Assess testing: which tests exist and what they cover, critical paths without tests (map them to features in the intent and specs), test quality (do assertions actually fail when behaviour breaks?), how tests are run (commands, CI gating), and flaky or environment-dependent tests. Do not run the tests; infer from the files.`,
	},
	{
		ID:           "ask",
		Title:        "Question",
		Description:  "Ask a read-only agent anything about the project.",
		Instructions: `Answer the owner's question about this project. Be direct, cite files, and say what you checked when the answer is uncertain.`,
	},
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

// ReviewKindIDs are the kinds that produce a verdict (everything but ask).
func ReviewKindIDs() []string {
	ids := []string{}
	for _, kind := range Kinds {
		if kind.ID != "ask" {
			ids = append(ids, kind.ID)
		}
	}
	return ids
}
