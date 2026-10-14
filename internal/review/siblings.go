package review

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ffqa/xpier-xplan/internal/project"
	"github.com/ffqa/xpier-xplan/internal/scanner"
)

// Limits for the other-project context of a drift review.
const (
	maxSiblings    = 8
	capSiblingGoal = 280
	capSiblingText = 160
)

// SiblingProject is a compact view of another registered project, so a drift
// review can tell which project a leaked concept came from.
type SiblingProject struct {
	Name         string
	Summary      string
	Goal         string
	Stack        []string
	Tags         []string
	LastActivity time.Time
}

// siblingProjects lists the other registered projects, most recently active
// first. Projects the owner hopped between lately are the likeliest sources
// of contamination, so they come first when the list is capped.
func (m *Manager) siblingProjects(currentSlug string) []SiblingProject {
	projectList, err := m.store.List()
	if err != nil {
		m.logger.Printf("drift context: list projects: %v", err)
		return nil
	}
	siblings := []SiblingProject{}
	for _, item := range projectList {
		if item.Slug == currentSlug {
			continue
		}
		siblings = append(siblings, m.describeSibling(item))
	}
	sort.SliceStable(siblings, func(i, j int) bool {
		return siblings[i].LastActivity.After(siblings[j].LastActivity)
	})
	if len(siblings) > maxSiblings {
		siblings = siblings[:maxSiblings]
	}
	return siblings
}

func (m *Manager) describeSibling(item *project.Project) SiblingProject {
	sibling := SiblingProject{
		Name:         item.Name,
		Summary:      clip(strings.Join(strings.Fields(item.Summary), " "), capSiblingText),
		Tags:         item.Tags,
		LastActivity: item.UpdatedAt,
	}
	if intent, err := m.store.ReadIntent(item.Slug); err == nil {
		sibling.Goal = project.IntentGoal(intent, capSiblingGoal)
	} else {
		m.logger.Printf("drift context: read intent %s: %v", item.Slug, err)
	}
	snapshot, err := scanner.LoadLatest(m.store.Dir(item.Slug))
	if err != nil || snapshot == nil {
		return sibling
	}
	sibling.Stack = snapshot.Stack
	if git := snapshot.Git; git != nil && git.LastCommitAt != nil && git.LastCommitAt.After(sibling.LastActivity) {
		sibling.LastActivity = *git.LastCommitAt
	}
	return sibling
}

func writeSiblings(b *strings.Builder, siblings []SiblingProject) {
	if len(siblings) == 0 {
		return
	}
	b.WriteString("### Other projects the owner develops in parallel (context for the contamination check only; do not review them)\n\n")
	for _, sibling := range siblings {
		fmt.Fprintf(b, "- **%s**", sibling.Name)
		if sibling.Summary != "" {
			fmt.Fprintf(b, " — %s", sibling.Summary)
		}
		b.WriteString("\n")
		if sibling.Goal != "" {
			fmt.Fprintf(b, "  Goal: %s\n", sibling.Goal)
		}
		var facts []string
		if len(sibling.Stack) > 0 {
			facts = append(facts, "stack "+strings.Join(sibling.Stack, ", "))
		}
		if len(sibling.Tags) > 0 {
			facts = append(facts, "tags "+strings.Join(sibling.Tags, ", "))
		}
		if !sibling.LastActivity.IsZero() {
			facts = append(facts, "last active "+sibling.LastActivity.Format("2006-01-02"))
		}
		if len(facts) > 0 {
			fmt.Fprintf(b, "  %s\n", strings.Join(facts, "; "))
		}
	}
	b.WriteString("\n")
}
