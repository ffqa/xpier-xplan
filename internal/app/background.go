package app

import (
	"context"
	"time"

	"github.com/ffqa/xplan/internal/insight"
	"github.com/ffqa/xplan/internal/review"
)

// schedulerTick is how often the background loop wakes up.
const schedulerTick = time.Minute

// graceAfterRegister delays the first scheduled review of a new project so
// that importing several projects does not fire a burst of agent runs.
const graceAfterRegister = time.Hour

// RunBackground periodically rescans projects whose snapshot is older than
// the scan interval and, when enabled, enqueues due scheduled reviews (at most
// one per tick to spread the load). It returns when ctx is canceled.
func (a *App) RunBackground(ctx context.Context, scheduleReviews bool) {
	a.Reviews.RecoverInterrupted()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			a.scanDue()
			if scheduleReviews {
				a.enqueueDueReview()
			}
			timer.Reset(schedulerTick)
		}
	}
}

func (a *App) scanDue() {
	projectList, err := a.Store.List()
	if err != nil {
		a.Logger.Printf("background: list projects: %v", err)
		return
	}
	interval := a.Config().ScanEvery()
	for _, item := range projectList {
		state, err := a.Store.ReadState(item.Slug)
		if err != nil {
			a.Logger.Printf("background: read state %s: %v", item.Slug, err)
			continue
		}
		if state.LastScanAt != nil && time.Since(*state.LastScanAt) < interval {
			continue
		}
		if _, err := a.ScanProject(item.Slug); err != nil {
			a.Logger.Printf("background: scan %s: %v", item.Slug, err)
		}
	}
}

// enqueueDueReview starts the single most overdue scheduled review, if any.
func (a *App) enqueueDueReview() {
	projectList, err := a.Store.List()
	if err != nil {
		a.Logger.Printf("scheduler: list projects: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, item := range projectList {
		if item.Status == "done" || item.Status == "paused" || now.Sub(item.CreatedAt) < graceAfterRegister {
			continue
		}
		for _, kind := range review.ReviewKindIDs() {
			period := insight.SchedulePeriod(item.Schedule[kind])
			if period == 0 {
				continue
			}
			last := a.Reviews.LastAttempt(item.Slug, kind)
			if last != nil && (last.Active() || now.Sub(last.CreatedAt) < period) {
				continue
			}
			if _, err := a.Reviews.Enqueue(item.Slug, kind, "", "schedule"); err != nil {
				a.Logger.Printf("scheduler: enqueue %s/%s: %v", item.Slug, kind, err)
				continue
			}
			a.Logger.Printf("scheduler: enqueued %s review for %s", kind, item.Slug)
			return
		}
	}
}
