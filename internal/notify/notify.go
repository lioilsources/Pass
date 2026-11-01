// Package notify posts a RevenueCat purchase event to Discord (webhook embed)
// and optionally Telegram. One attempt each, short timeout, no retry: the
// webhook handler records the outcome on the event row and a retry loop is a
// TODO for a later version (see store.MarkNotified).
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lioilsources/Pass/internal/catalog"
	"github.com/lioilsources/Pass/internal/revenuecat"
)

// Notifier holds the sinks. Empty URL/token disables that sink.
type Notifier struct {
	DiscordWebhookURL string
	TelegramBotToken  string
	TelegramChatID    string
	TelegramBaseURL   string // for tests; default https://api.telegram.org
	HTTP              *http.Client
	Catalog           *catalog.Catalog
}

// New builds a notifier with a 10 s HTTP timeout.
func New(cat *catalog.Catalog) *Notifier {
	return &Notifier{
		HTTP:            &http.Client{Timeout: 10 * time.Second},
		TelegramBaseURL: "https://api.telegram.org",
		Catalog:         cat,
	}
}

// Enabled reports whether at least one sink is configured.
func (n *Notifier) Enabled() bool {
	return n.DiscordWebhookURL != "" || (n.TelegramBotToken != "" && n.TelegramChatID != "")
}

const (
	colorGreen  = 0x2F7D32 // matches the olin.now accent
	colorRed    = 0xC62828
	colorOrange = 0xEF6C00
	colorGrey   = 0x6C7160
)

// Color picks the embed colour for an event type.
func Color(eventType string) int {
	switch eventType {
	case "INITIAL_PURCHASE", "NON_RENEWING_PURCHASE", "RENEWAL":
		return colorGreen
	case "CANCELLATION", "EXPIRATION":
		return colorRed
	case "BILLING_ISSUE":
		return colorOrange
	default:
		return colorGrey
	}
}

// Title is the one-line headline shared by both sinks.
func Title(e revenuecat.Event) string {
	switch e.Type {
	case "INITIAL_PURCHASE", "NON_RENEWING_PURCHASE":
		return "Ol1n Pass: new purchase"
	case "RENEWAL":
		return "Ol1n Pass: renewal"
	case "CANCELLATION":
		return "Ol1n Pass: cancellation / refund"
	case "EXPIRATION":
		return "Ol1n Pass: expired"
	case "BILLING_ISSUE":
		return "Ol1n Pass: billing issue"
	case "TRANSFER":
		return "Ol1n Pass: transferred to a new device"
	case "PRODUCT_CHANGE":
		return "Ol1n Pass: product change"
	default:
		return "Ol1n Pass: " + e.Type
	}
}

// Field is one labelled value of the message.
type Field struct{ Name, Value string }

// Fields renders the event for humans. The app is resolved from the iOS
// product id; Android SKUs are shared, so there the field says so.
func (n *Notifier) Fields(e revenuecat.Event) []Field {
	app := "?"
	if n.Catalog != nil {
		if slug, ok := n.Catalog.SlugForProduct(e.ProductID); ok {
			app = slug
		} else if e.ProductID == catalog.AndroidLifetimeSKU || e.ProductID == catalog.AndroidYearlySKU {
			app = "(Android, see RevenueCat app)"
		}
	}
	kind := "lifetime"
	if strings.Contains(e.ProductID, "yearly") {
		kind = "yearly"
	}
	price := "—"
	if e.Price != 0 || e.Currency != "" {
		price = fmt.Sprintf("%.2f %s", e.Price, e.Currency)
	}
	return []Field{
		{"App", app},
		{"Store", strings.ToLower(strings.ReplaceAll(e.Store, "_", " "))},
		{"Product", fmt.Sprintf("%s (%s)", e.ProductID, kind)},
		{"Price", price},
		{"Environment", strings.ToLower(e.Environment)},
		{"User", shortID(e.AppUserID)},
	}
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…" + id[len(id)-4:]
}

// Send posts to every configured sink and returns the joined errors.
func (n *Notifier) Send(ctx context.Context, e revenuecat.Event) error {
	var errs []error
	if n.DiscordWebhookURL != "" {
		if err := n.sendDiscord(ctx, e); err != nil {
			errs = append(errs, err)
		}
	}
	if n.TelegramBotToken != "" && n.TelegramChatID != "" {
		if err := n.sendTelegram(ctx, e); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type discordEmbed struct {
	Title     string         `json:"title"`
	Color     int            `json:"color"`
	Fields    []discordField `json:"fields"`
	Timestamp string         `json:"timestamp,omitempty"`
	Footer    *struct {
		Text string `json:"text"`
	} `json:"footer,omitempty"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// DiscordPayload builds the webhook body (exported for tests).
func (n *Notifier) DiscordPayload(e revenuecat.Event) map[string]any {
	emb := discordEmbed{Title: Title(e), Color: Color(e.Type)}
	for _, f := range n.Fields(e) {
		emb.Fields = append(emb.Fields, discordField{Name: f.Name, Value: f.Value, Inline: true})
	}
	if !e.EventTimestamp.IsZero() {
		emb.Timestamp = e.EventTimestamp.UTC().Format(time.RFC3339)
	}
	emb.Footer = &struct {
		Text string `json:"text"`
	}{Text: "RevenueCat " + e.Type + " · " + e.ID}
	return map[string]any{"embeds": []discordEmbed{emb}}
}

func (n *Notifier) sendDiscord(ctx context.Context, e revenuecat.Event) error {
	return n.postJSON(ctx, n.DiscordWebhookURL, n.DiscordPayload(e), "discord")
}

// TelegramText renders the plain-text message.
func (n *Notifier) TelegramText(e revenuecat.Event) string {
	var sb strings.Builder
	sb.WriteString(Title(e))
	for _, f := range n.Fields(e) {
		fmt.Fprintf(&sb, "\n%s: %s", f.Name, f.Value)
	}
	return sb.String()
}

func (n *Notifier) sendTelegram(ctx context.Context, e revenuecat.Event) error {
	u := strings.TrimRight(n.TelegramBaseURL, "/") + "/bot" + url.PathEscape(n.TelegramBotToken) + "/sendMessage"
	body := map[string]any{"chat_id": n.TelegramChatID, "text": n.TelegramText(e), "disable_web_page_preview": true}
	return n.postJSON(ctx, u, body, "telegram")
}

func (n *Notifier) postJSON(ctx context.Context, u string, body any, sink string) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("notify: %s: %w", sink, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("notify: %s: %s: %s", sink, resp.Status, strings.TrimSpace(string(rb)))
	}
	return nil
}
