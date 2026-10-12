package insight

import (
	"strings"
	"time"

	"github.com/ffqa/xplan/internal/project"
	"github.com/ffqa/xplan/internal/review"
	"github.com/ffqa/xplan/internal/scanner"
)

// Card is the compact project summary used by the sidebar and the board.
type Card struct {
	*project.Project
	HealthLevel       string            `json:"health_level"`
	Health            []Flag            `json:"health"`
	IntentFilled      bool              `json:"intent_filled"`
	IntentDraft       bool              `json:"intent_draft"`
	Established       bool              `json:"established"`
	Branch            string            `json:"branch,omitempty"`
	LastCommitAt      *time.Time        `json:"last_commit_at,omitempty"`
	LastCommitSubject string            `json:"last_commit_subject,omitempty"`
	DirtyFiles        int               `json:"dirty_files"`
	Commits7d         int               `json:"commits_7d"`
	Checklist         scanner.Checklist `json:"checklist"`
	XteamOpen         int               `json:"xteam_open"`
	XteamClosed       int               `json:"xteam_closed"`
	ChangelogVersion  string            `json:"changelog_version,omitempty"`
	DocCount          int               `json:"doc_count"`
	Stack             []string          `json:"stack"`
	LastPark          *project.Entry    `json:"last_park,omitempty"`
	LastScanAt        *time.Time        `json:"last_scan_at,omitempty"`
	ActiveJobs        int               `json:"active_jobs"`
	Verdicts          map[string]string `json:"verdicts"`
}

// BuildCard assembles a card from facts.
func BuildCard(facts Facts) Card {
	flags := Health(facts)
	card := Card{
		Project:      facts.Project,
		HealthLevel:  MaxLevel(flags),
		Health:       flags,
		IntentFilled: facts.IntentFilled,
		IntentDraft:  facts.IntentDraft,
		Established:  Established(facts.Project, facts.Snapshot),
		Stack:        []string{},
		LastPark:     facts.LastPark,
		ActiveJobs:   facts.ActiveJobs,
		Verdicts:     map[string]string{},
	}
	for kind, job := range facts.LatestDone {
		card.Verdicts[kind] = job.Verdict
	}
	if snapshot := facts.Snapshot; snapshot != nil {
		scannedAt := snapshot.ScannedAt
		card.LastScanAt = &scannedAt
		card.Checklist = snapshot.Checklist
		card.DocCount = len(snapshot.Docs)
		card.Stack = snapshot.Stack
		if git := snapshot.Git; git != nil {
			card.Branch = git.Branch
			card.LastCommitAt = git.LastCommitAt
			card.DirtyFiles = git.DirtyFiles
			card.Commits7d = git.Commits7d
			if len(git.Recent) > 0 {
				card.LastCommitSubject = git.Recent[0].Subject
			}
		}
		if snapshot.Xteam != nil {
			card.XteamOpen = snapshot.Xteam.Open
			card.XteamClosed = snapshot.Xteam.Closed
		}
		if snapshot.Changelog != nil {
			card.ChangelogVersion = snapshot.Changelog.Version
		}
	}
	return card
}

// Since describes what changed after the last visit.
type Since struct {
	Reference  string              `json:"reference"`
	At         *time.Time          `json:"at,omitempty"`
	Head       string              `json:"head,omitempty"`
	Commits    []scanner.Commit    `json:"commits"`
	Docs       []scanner.Doc       `json:"docs"`
	Slices     []scanner.XteamTask `json:"slices"`
	Reviews    []*review.Job       `json:"reviews"`
	OtherFiles int                 `json:"other_files"`
}

// Brief is the re-entry view: everything needed to pick a project back up.
type Brief struct {
	Card       Card                   `json:"card"`
	Intent     string                 `json:"intent"`
	ADRs       []project.ADR          `json:"adrs"`
	LastVisit  *project.Visit         `json:"last_visit,omitempty"`
	Since      Since                  `json:"since"`
	Snapshot   *scanner.Snapshot      `json:"snapshot,omitempty"`
	LatestDone map[string]*review.Job `json:"latest_reviews"`
	Active     []*review.Job          `json:"active_reviews"`
}

// BriefInput carries the extra data only the brief needs.
type BriefInput struct {
	Facts   Facts
	Intent  string
	ADRs    []project.ADR
	Reviews []*review.Job
}

const briefCommitLimit = 30

// BuildBrief assembles the re-entry brief. The reference point is the later of
// the last visit and the last park note, because both mean "I knew the state
// of things at that moment".
func BuildBrief(input BriefInput) Brief {
	facts := input.Facts
	card := BuildCard(facts)
	brief := Brief{
		Card:       card,
		Intent:     input.Intent,
		ADRs:       input.ADRs,
		LastVisit:  facts.State.LastVisit,
		Snapshot:   facts.Snapshot,
		LatestDone: facts.LatestDone,
		Active:     []*review.Job{},
		Since:      Since{Commits: []scanner.Commit{}, Docs: []scanner.Doc{}, Slices: []scanner.XteamTask{}, Reviews: []*review.Job{}},
	}
	for _, job := range input.Reviews {
		if job.Active() {
			brief.Active = append(brief.Active, job)
		}
	}

	referenceAt, referenceHead, referenceLabel := referencePoint(facts)
	brief.Since.Reference = referenceLabel
	if referenceAt == nil {
		return brief
	}
	brief.Since.At = referenceAt
	brief.Since.Head = referenceHead

	for _, job := range input.Reviews {
		if review.IsReviewKind(job.Kind) && job.Status == review.StatusDone && job.CreatedAt.After(*referenceAt) {
			brief.Since.Reviews = append(brief.Since.Reviews, job)
		}
	}
	snapshot := facts.Snapshot
	if snapshot == nil || !snapshot.PathExists {
		return brief
	}
	if snapshot.Git != nil {
		if commits, err := scanner.CommitsSince(facts.Project.Path, referenceHead, *referenceAt, briefCommitLimit); err == nil {
			brief.Since.Commits = commits
		}
		changed := scanner.ChangedFilesSince(facts.Project.Path, referenceHead)
		docPaths := map[string]bool{}
		for _, doc := range snapshot.Docs {
			docPaths[doc.Path] = true
		}
		changedSet := map[string]bool{}
		for _, path := range changed {
			changedSet[path] = true
			if !docPaths[path] {
				brief.Since.OtherFiles++
			}
		}
		for _, doc := range snapshot.Docs {
			if changedSet[doc.Path] || (referenceHead == "" && doc.ModTime.After(*referenceAt)) {
				brief.Since.Docs = append(brief.Since.Docs, doc)
			}
		}
	} else {
		for _, doc := range snapshot.Docs {
			if doc.ModTime.After(*referenceAt) {
				brief.Since.Docs = append(brief.Since.Docs, doc)
			}
		}
	}
	if snapshot.Xteam != nil {
		for _, task := range snapshot.Xteam.Tasks {
			if task.UpdatedAt.After(*referenceAt) {
				brief.Since.Slices = append(brief.Since.Slices, task)
			}
		}
	}
	return brief
}

func referencePoint(facts Facts) (*time.Time, string, string) {
	var at *time.Time
	head := ""
	label := ""
	if visit := facts.State.LastVisit; visit != nil {
		visitAt := visit.At
		at, head, label = &visitAt, visit.Head, "last visit"
	}
	if park := facts.LastPark; park != nil && (at == nil || park.At.After(*at)) {
		parkAt := park.At
		at, head, label = &parkAt, park.Head, "last park note"
	}
	return at, strings.TrimSpace(head), label
}
