package server

import (
	"encoding/json"
	"strings"

	"github.com/Norrodar/TidyDAV/internal/notify"
	"github.com/Norrodar/TidyDAV/internal/store"
)

type notificationsDTO struct {
	WebhookURL   string   `json:"webhookUrl,omitempty"`
	NtfyServer   string   `json:"ntfyServer,omitempty"`
	NtfyTopic    string   `json:"ntfyTopic,omitempty"`
	GotifyServer string   `json:"gotifyServer,omitempty"`
	GotifyToken  string   `json:"gotifyToken,omitempty"` // write-only
	Triggers     []string `json:"triggers,omitempty"`
}

type notificationsResponse struct {
	WebhookURL     string   `json:"webhookUrl"`
	NtfyServer     string   `json:"ntfyServer"`
	NtfyTopic      string   `json:"ntfyTopic"`
	GotifyServer   string   `json:"gotifyServer"`
	GotifyTokenSet bool     `json:"gotifyTokenSet"`
	Triggers       []string `json:"triggers"`
}

// buildNotifications produces the stored JSON from a request, preserving the
// Gotify token across updates when the request omits it (it is write-only).
func buildNotifications(req *notificationsDTO, existing *store.Feed) (json.RawMessage, error) {
	if req == nil {
		if existing != nil && len(existing.Notifications) > 0 {
			return existing.Notifications, nil
		}
		return json.RawMessage("{}"), nil
	}
	n := notify.FeedNotifications{
		WebhookURL:   strings.TrimSpace(req.WebhookURL),
		NtfyServer:   strings.TrimSpace(req.NtfyServer),
		NtfyTopic:    strings.TrimSpace(req.NtfyTopic),
		GotifyServer: strings.TrimSpace(req.GotifyServer),
		GotifyToken:  req.GotifyToken,
		Triggers:     req.Triggers,
	}
	if n.GotifyToken == "" && existing != nil {
		if prev := parseNotifications(existing.Notifications); prev.GotifyToken != "" {
			n.GotifyToken = prev.GotifyToken
		}
	}
	b, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func parseNotifications(raw json.RawMessage) notify.FeedNotifications {
	var n notify.FeedNotifications
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &n)
	}
	return n
}

func toNotificationsResponse(raw json.RawMessage) notificationsResponse {
	n := parseNotifications(raw)
	return notificationsResponse{
		WebhookURL:     n.WebhookURL,
		NtfyServer:     n.NtfyServer,
		NtfyTopic:      n.NtfyTopic,
		GotifyServer:   n.GotifyServer,
		GotifyTokenSet: n.GotifyToken != "",
		Triggers:       n.Triggers,
	}
}
