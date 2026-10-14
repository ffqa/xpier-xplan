package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// webhookFormat picks the payload shape from the webhook's host, so pasting
// a Slack, Feishu/Lark, DingTalk, WeCom or Discord bot URL just works.
func webhookFormat(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "generic"
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "hooks.slack.com":
		return "slack"
	case strings.HasSuffix(host, "feishu.cn") || strings.HasSuffix(host, "larksuite.com"):
		return "feishu"
	case host == "oapi.dingtalk.com":
		return "dingtalk"
	case host == "qyapi.weixin.qq.com":
		return "wecom"
	case (host == "discord.com" || host == "discordapp.com") && strings.Contains(parsed.Path, "/webhooks/"):
		return "discord"
	default:
		return "generic"
	}
}

// webhookPayload renders message for the given format. The generic shape has
// a Slack-style "text" (understood by Mattermost, Rocket.Chat and others)
// plus structured fields for custom receivers.
func webhookPayload(format string, message Message) any {
	text := message.Title + "\n" + message.Body
	if message.URL != "" {
		text += "\n" + message.URL
	}
	switch format {
	case "slack":
		return map[string]string{"text": text}
	case "feishu":
		return map[string]any{"msg_type": "text", "content": map[string]string{"text": text}}
	case "dingtalk", "wecom":
		return map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}
	case "discord":
		return map[string]string{"content": text}
	default:
		return map[string]any{
			"text":    text,
			"title":   message.Title,
			"body":    message.Body,
			"url":     message.URL,
			"project": message.Project,
			"kind":    message.Kind,
			"status":  message.Status,
			"verdict": message.Verdict,
			"score":   message.Score,
		}
	}
}

// chatError is how Feishu, DingTalk and WeCom report failures inside a 200.
type chatError struct {
	Code    *int   `json:"code"`
	ErrCode *int   `json:"errcode"`
	Msg     string `json:"msg"`
	ErrMsg  string `json:"errmsg"`
}

func postWebhook(ctx context.Context, client *http.Client, rawURL string, message Message) error {
	body, err := json.Marshal(webhookPayload(webhookFormat(rawURL), message))
	if err != nil {
		return fmt.Errorf("encode webhook payload: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("post webhook: %w", redactURL(err, rawURL))
	}
	defer response.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("webhook answered %d: %s", response.StatusCode, strings.TrimSpace(string(answer)))
	}
	var reply chatError
	if json.Unmarshal(answer, &reply) == nil {
		if code := firstNonZero(reply.Code, reply.ErrCode); code != 0 {
			return fmt.Errorf("webhook rejected the message (code %d): %s", code, strings.TrimSpace(reply.Msg+reply.ErrMsg))
		}
	}
	return nil
}

func firstNonZero(values ...*int) int {
	for _, value := range values {
		if value != nil && *value != 0 {
			return *value
		}
	}
	return 0
}

// redactURL keeps the webhook's secret path and query out of error messages
// that end up in logs and API responses.
func redactURL(err error, rawURL string) error {
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return err
	}
	safe := parsed.Scheme + "://" + parsed.Host + "/…"
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), rawURL, safe))
}

// MaskURL shows enough of a webhook URL to recognize it without revealing it.
func MaskURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "…"
	}
	tail := rawURL
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	return parsed.Scheme + "://" + parsed.Host + "/…" + tail
}
