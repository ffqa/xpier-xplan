package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const siblingsHeading = "Other projects the owner develops in parallel"

// TestDriftPromptListsOtherProjects checks that a drift review tells the agent
// what the owner's other projects are about, and that other kinds do not.
func TestDriftPromptListsOtherProjects(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()

	shopDir := t.TempDir()
	writeFile(t, shopDir, "README.md", "# Shop\n\nA storefront.\n")
	shop := env.expect("POST", "/api/projects", map[string]string{"path": shopDir, "name": "Shop"}, http.StatusCreated)
	shopSlug, _ := shop["slug"].(string)
	env.expect("PATCH", "/api/projects/"+shopSlug, map[string]any{"summary": "Online storefront with carts and coupons"}, http.StatusOK)
	shopIntent := "# Intent — Shop\n\n## Why (the problem)\n- Selling needs a cart.\n\n## Goal (north star)\n- Sell posters with coupon codes\n- Ship within a day\n"
	env.expect("PUT", "/api/projects/"+shopSlug+"/intent", map[string]string{"content": shopIntent}, http.StatusOK)

	driftPrompt := env.promptOf(slug, "drift")
	for _, want := range []string{siblingsHeading, "**Shop**", "Online storefront with carts and coupons", "Goal: Sell posters with coupon codes; Ship within a day", "Name the project each one seems to come from"} {
		if !strings.Contains(driftPrompt, want) {
			t.Fatalf("drift prompt is missing %q:\n%s", want, driftPrompt)
		}
	}
	if strings.Contains(driftPrompt, "**Demo**") {
		t.Fatal("the reviewed project must not list itself as another project")
	}

	if codePrompt := env.promptOf(slug, "code"); strings.Contains(codePrompt, siblingsHeading) {
		t.Fatal("only drift reviews need the other projects")
	}
}

// promptOf runs a review of kind on slug and returns the prompt xplan saved.
func (e *testEnv) promptOf(slug, kind string) string {
	e.t.Helper()
	base := "/api/projects/" + slug
	started := e.expect("POST", base+"/reviews", map[string]string{"kind": kind}, http.StatusAccepted)
	jobID, _ := started["id"].(string)
	if job := waitForJob(e.t, e, base, jobID); job["status"] != "done" {
		e.t.Fatalf("%s review did not finish: %v", kind, job)
	}
	raw, err := os.ReadFile(filepath.Join(e.app.Store.Dir(slug), "reviews", jobID+".prompt.md"))
	if err != nil {
		e.t.Fatalf("read prompt: %v", err)
	}
	return string(raw)
}
