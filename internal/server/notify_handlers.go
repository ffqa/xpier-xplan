package server

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/desktop"
	"github.com/ffqa/xpier-xplan/internal/notify"
	"github.com/ffqa/xpier-xplan/internal/project"
)

// notifyView is what the API shows of the notification settings: the
// webhook URL itself never leaves the server, only a recognizable hint.
type notifyView struct {
	Desktop          bool   `json:"desktop"`
	DesktopAvailable bool   `json:"desktop_available"`
	MinVerdict       string `json:"min_verdict"`
	WebhookSet       bool   `json:"webhook_set"`
	WebhookHint      string `json:"webhook_hint"`
}

// configView is the config as returned by GET and PATCH /api/config.
type configView struct {
	*config.Config
	Notify notifyView `json:"notify"`
}

func newConfigView(cfg *config.Config) configView {
	return configView{
		Config: cfg,
		Notify: notifyView{
			Desktop:          cfg.Notify.Desktop,
			DesktopAvailable: desktop.NotifyAvailable(),
			MinVerdict:       cfg.Notify.MinVerdict,
			WebhookSet:       cfg.Notify.WebhookURL != "",
			WebhookHint:      notify.MaskURL(cfg.Notify.WebhookURL),
		},
	}
}

// notifyPatch carries optional notification changes. webhook_url "" removes
// the webhook; leaving it out keeps the current one.
type notifyPatch struct {
	Desktop    *bool   `json:"desktop"`
	MinVerdict *string `json:"min_verdict"`
	WebhookURL *string `json:"webhook_url"`
}

func applyNotifyPatch(settings *config.NotifySettings, patch *notifyPatch) error {
	if patch == nil {
		return nil
	}
	if patch.Desktop != nil {
		settings.Desktop = *patch.Desktop
	}
	if patch.MinVerdict != nil {
		if !slices.Contains(config.NotifyLevels, *patch.MinVerdict) {
			return fmt.Errorf("%w: min_verdict must be one of %v", project.ErrInvalid, config.NotifyLevels)
		}
		settings.MinVerdict = *patch.MinVerdict
	}
	if patch.WebhookURL != nil {
		webhook := strings.TrimSpace(*patch.WebhookURL)
		if webhook != "" {
			if err := config.ValidateWebhookURL(webhook); err != nil {
				return fmt.Errorf("%w: %v", project.ErrInvalid, err)
			}
		}
		settings.WebhookURL = webhook
	}
	return nil
}

func (s *Server) handleTestNotify(w http.ResponseWriter, r *http.Request) {
	results, err := s.app.Notifier.Test()
	if errors.Is(err, notify.ErrNoChannel) {
		s.writeError(w, fmt.Errorf("%w: %v", project.ErrInvalid, err))
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
