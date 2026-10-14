package app

import (
	"sort"
	"time"

	"github.com/ffqa/xpier-xplan/internal/review"
)

// InboxItem is one finding from the latest finished review of a kind.
type InboxItem struct {
	Project     string         `json:"project"`
	ProjectName string         `json:"project_name"`
	Kind        string         `json:"kind"`
	JobID       string         `json:"job_id"`
	Verdict     string         `json:"verdict"`
	At          time.Time      `json:"at"`
	Finding     review.Finding `json:"finding"`
}

// SeverityRank orders finding severities; unknown severities rank as info.
var SeverityRank = map[string]int{"critical": 4, "high": 3, "medium": 2, "low": 1, "info": 0}

// Inbox lists findings from the latest finished review of every kind, most
// severe first. slug limits it to one project; minSeverity drops lesser ones.
func (a *App) Inbox(slug, minSeverity string) ([]InboxItem, error) {
	projectList, err := a.Store.List()
	if err != nil {
		return nil, err
	}
	minimum := SeverityRank[minSeverity]
	items := []InboxItem{}
	for _, item := range projectList {
		if slug != "" && item.Slug != slug {
			continue
		}
		for kind, job := range a.Reviews.LatestDone(item.Slug) {
			for _, finding := range job.Findings {
				if SeverityRank[finding.Severity] < minimum {
					continue
				}
				items = append(items, InboxItem{Project: item.Slug, ProjectName: item.Name, Kind: kind, JobID: job.ID, Verdict: job.Verdict, At: job.CreatedAt, Finding: finding})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := SeverityRank[items[i].Finding.Severity], SeverityRank[items[j].Finding.Severity]
		if left != right {
			return left > right
		}
		return items[i].At.After(items[j].At)
	})
	return items, nil
}
