package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lioilsources/Pass/internal/catalog"
	"github.com/lioilsources/Pass/internal/revenuecat"
)

func TestSendBothSinks(t *testing.T) {
	var discordBody, telegramPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.HasPrefix(r.URL.Path, "/bot") {
			telegramPath = r.URL.Path
			if !strings.Contains(string(b), "Ol1n Pass: new purchase") {
				t.Errorf("telegram text: %s", b)
			}
			w.Write([]byte(`{"ok":true}`))
			return
		}
		discordBody = string(b)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	cat, _ := catalog.Default()
	n := New(cat)
	n.DiscordWebhookURL = srv.URL + "/discord"
	n.TelegramBotToken = "tok"
	n.TelegramChatID = "42"
	n.TelegramBaseURL = srv.URL

	e := revenuecat.Event{ID: "evt", Type: "INITIAL_PURCHASE", AppUserID: "0123456789abcdef", ProductID: "com.ol1n.pass.lifetime.kirian", Store: "APP_STORE", Price: 49, Currency: "USD", Environment: "PRODUCTION"}
	if err := n.Send(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Embeds []struct {
			Title  string
			Color  int
			Fields []struct{ Name, Value string }
		}
	}
	if err := json.Unmarshal([]byte(discordBody), &payload); err != nil {
		t.Fatal(err)
	}
	emb := payload.Embeds[0]
	if emb.Color != colorGreen || emb.Title != "Ol1n Pass: new purchase" {
		t.Fatalf("%+v", emb)
	}
	got := map[string]string{}
	for _, f := range emb.Fields {
		got[f.Name] = f.Value
	}
	if got["App"] != "kirian" || got["Price"] != "49.00 USD" || got["User"] != "01234567…cdef" {
		t.Fatalf("fields %v", got)
	}
	if telegramPath != "/bottok/sendMessage" {
		t.Fatal(telegramPath)
	}
}

func TestColorsAndFailure(t *testing.T) {
	if Color("CANCELLATION") != colorRed || Color("BILLING_ISSUE") != colorOrange || Color("TRANSFER") != colorGrey {
		t.Fatal("colors")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	n := New(nil)
	n.DiscordWebhookURL = srv.URL
	if err := n.Send(context.Background(), revenuecat.Event{Type: "RENEWAL", ProductID: "pass_yearly"}); err == nil {
		t.Fatal("expected error")
	}
	if New(nil).Enabled() {
		t.Fatal("no sinks must be disabled")
	}
}
