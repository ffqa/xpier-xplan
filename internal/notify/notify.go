// Package notify tells the owner when a review needs their attention: a
// verdict at or above the configured level, or a review that failed. It
// shows a desktop notification and/or posts to a chat webhook.
package notify

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/desktop"
	"github.com/ffqa/xpier-xplan/internal/i18n"
	"github.com/ffqa/xpier-xplan/internal/review"
)

// Channel names reported by Send.
const (
	ChannelDesktop = "desktop"
	ChannelWebhook = "webhook"
)

// ErrNoChannel is returned by Test when every channel is off.
var ErrNoChannel = errors.New("no notification channel is enabled")

// Message is one notification.
type Message struct {
	Title   string
	Body    string
	URL     string
	Project string
	Kind    string
	Status  string
	Verdict string
	Score   int
}

// Result is the outcome of one channel.
type Result struct {
	Channel string `json:"channel"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

// Notifier sends notifications according to the current settings.
type Notifier struct {
	config  func() *config.Config
	baseURL func() string
	logger  *log.Logger
	client  *http.Client
	desktop func(title, body string) error
}

// New creates a notifier; config is read at send time so edits apply at once.
// baseURL gives the address links in messages point to.
func New(cfg func() *config.Config, baseURL func() string, logger *log.Logger) *Notifier {
	return &Notifier{
		config:  cfg,
		baseURL: baseURL,
		logger:  logger,
		client:  &http.Client{Timeout: 15 * time.Second},
		desktop: desktop.Notify,
	}
}

// ReviewFinished is the review manager's hook. It runs after every job and
// sends a notification only when the outcome deserves one.
func (n *Notifier) ReviewFinished(job review.Job, projectName string) {
	settings := n.config().Notify
	if !ShouldNotify(settings, job) {
		return
	}
	message := reviewMessage(job, projectName, n.baseURL())
	for _, result := range n.Send(message) {
		if !result.OK {
			n.logger.Printf("notify %s failed project=%s id=%s: %s", result.Channel, job.Project, job.ID, result.Error)
		}
	}
}

// Test sends a sample notification to every enabled channel.
func (n *Notifier) Test() ([]Result, error) {
	settings := n.config().Notify
	if !settings.Desktop && settings.WebhookURL == "" {
		return nil, ErrNoChannel
	}
	return n.Send(Message{
		Title:   i18n.T("xplan test notification"),
		Body:    i18n.T("Notifications work. xplan will tell you here when a review needs attention."),
		URL:     n.baseURL() + "/#/settings",
		Kind:    "test",
		Status:  "test",
		Verdict: review.VerdictOK,
	}), nil
}

// Send delivers message to every enabled channel.
func (n *Notifier) Send(message Message) []Result {
	settings := n.config().Notify
	results := []Result{}
	if settings.Desktop {
		results = append(results, outcome(ChannelDesktop, n.desktop(message.Title, message.Body)))
	}
	if settings.WebhookURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		results = append(results, outcome(ChannelWebhook, postWebhook(ctx, n.client, settings.WebhookURL, message)))
		cancel()
	}
	return results
}

// ShouldNotify decides whether a finished job is worth a notification:
// review kinds only, when they failed or reached the configured verdict.
func ShouldNotify(settings config.NotifySettings, job review.Job) bool {
	if !review.IsReviewKind(job.Kind) {
		return false
	}
	switch job.Status {
	case review.StatusFailed:
		return true
	case review.StatusDone:
		return verdictRank(job.Verdict) >= verdictRank(settings.MinVerdict)
	default:
		return false
	}
}

// verdictRank orders verdicts; a report without a verdict counts as needing
// attention, since nobody knows what it says until they read it.
func verdictRank(verdict string) int {
	switch verdict {
	case review.VerdictOK:
		return 0
	case review.VerdictCritical:
		return 2
	default:
		return 1
	}
}

func reviewMessage(job review.Job, projectName, baseURL string) Message {
	message := Message{
		Title:   i18n.T("xplan · %s · %s", projectName, review.KindTitle(job.Kind)),
		URL:     fmt.Sprintf("%s/#/p/%s/reviews/%s", baseURL, job.Project, job.ID),
		Project: job.Project,
		Kind:    job.Kind,
		Status:  job.Status,
		Verdict: job.Verdict,
		Score:   job.Score,
	}
	if job.Status == review.StatusFailed {
		message.Body = i18n.T("Review failed: %s", firstLine(job.Error))
		return message
	}
	label := VerdictLabel(job.Kind, job.Verdict)
	if job.Summary == "" {
		message.Body = i18n.T("%s, score %d", label, job.Score)
		return message
	}
	message.Body = i18n.T("%s, score %d: %s", label, job.Score, firstLine(job.Summary))
	return message
}

// VerdictLabel names a verdict the way the UI does; drift reviews speak of
// alignment rather than quality.
func VerdictLabel(kind, verdict string) string {
	if kind == review.KindDrift {
		switch verdict {
		case review.VerdictOK:
			return i18n.T("aligned")
		case review.VerdictAttention:
			return i18n.T("drifting")
		case review.VerdictCritical:
			return i18n.T("off-track")
		}
	}
	switch verdict {
	case review.VerdictOK:
		return i18n.T("ok")
	case review.VerdictAttention:
		return i18n.T("attention")
	case review.VerdictCritical:
		return i18n.T("critical")
	default:
		return i18n.T("unknown")
	}
}

func outcome(channel string, err error) Result {
	if err != nil {
		return Result{Channel: channel, Error: err.Error()}
	}
	return Result{Channel: channel, OK: true}
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if index := strings.Index(text, "\n"); index >= 0 {
		text = text[:index]
	}
	runes := []rune(text)
	if len(runes) > 300 {
		return string(runes[:299]) + "…"
	}
	return text
}
