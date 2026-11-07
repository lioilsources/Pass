// pass-api is the small server behind Ol1n Pass (docs/CONTRACTS.md §4): it
// serves the app catalog, answers entitlement lookups, issues and redeems link
// codes and turns RevenueCat webhooks into Discord/Telegram messages.
//
// Subcommands: serve (default), migrate, healthcheck.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lioilsources/Pass/internal/catalog"
	"github.com/lioilsources/Pass/internal/httpapi"
	"github.com/lioilsources/Pass/internal/notify"
	"github.com/lioilsources/Pass/internal/revenuecat"
	"github.com/lioilsources/Pass/internal/store"
)

var version = "dev"

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel()}))
	slog.SetDefault(logger)

	var err error
	switch cmd {
	case "serve":
		err = serve(logger)
	case "migrate":
		err = migrate()
	case "healthcheck":
		err = healthcheck()
	case "version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q (serve|migrate|healthcheck|version)", cmd)
	}
	if err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func logLevel() slog.Level {
	switch strings.ToLower(env("LOG_LEVEL", "info")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return slog.LevelInfo
}

func openStore() (*store.Store, error) {
	st, err := store.Open(env("PASS_DB_PATH", "/data/pass.db"))
	if err != nil {
		return nil, err
	}
	if err := st.Migrate(); err != nil {
		st.Close()
		return nil, err
	}
	return st, nil
}

func migrate() error {
	st, err := openStore()
	if err != nil {
		return err
	}
	return st.Close()
}

func loadCatalog() (*catalog.Catalog, []byte, error) {
	if p := os.Getenv("PASS_CATALOG_PATH"); p != "" {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, nil, err
		}
		c, err := catalog.Parse(raw)
		return c, raw, err
	}
	c, err := catalog.Default()
	return c, catalog.DefaultJSON(), err
}

func serve(logger *slog.Logger) error {
	cat, raw, err := loadCatalog()
	if err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	// Without a RevenueCat key the server still serves health and the catalog;
	// every entitlement check answers 503, so nobody is ever granted a Pass.
	var rc httpapi.Entitlements = revenuecat.Unconfigured{}
	if rcKey := os.Getenv("RC_SECRET_API_KEY"); rcKey != "" {
		rc = revenuecat.New(rcKey)
	} else {
		logger.Warn("RC_SECRET_API_KEY is empty; entitlement checks and link codes answer 503")
	}

	n := notify.New(cat)
	n.DiscordWebhookURL = os.Getenv("DISCORD_WEBHOOK_URL")
	n.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	n.TelegramChatID = os.Getenv("TELEGRAM_CHAT_ID")
	if !n.Enabled() {
		logger.Warn("no notification sink configured (DISCORD_WEBHOOK_URL / TELEGRAM_*); webhook events are only stored")
	}
	whSecret := os.Getenv("RC_WEBHOOK_SECRET")
	if whSecret == "" {
		logger.Warn("RC_WEBHOOK_SECRET is empty; /webhooks/revenuecat answers 501")
	}

	h := httpapi.New(httpapi.Config{
		Catalog: cat, CatalogJSON: raw, Store: st, Entitlements: rc, Notifier: n,
		WebhookSecret: whSecret, Logger: logger,
	})
	addr := env("PASS_ADDR", ":8096")
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Housekeeping: drop expired link codes every hour.
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n, err := st.PurgeExpiredLinkCodes(time.Now().Add(-time.Hour)); err != nil {
					logger.Error("purge link codes", "err", err)
				} else if n > 0 {
					logger.Info("purged link codes", "n", n)
				}
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("pass-api listening", "addr", addr, "version", version, "apps", len(cat.Apps))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info("shutting down")
	return srv.Shutdown(shutdownCtx)
}

// healthcheck is what the Docker HEALTHCHECK runs inside the distroless image.
func healthcheck() error {
	addr := env("PASS_ADDR", ":8096")
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "" {
		host = "127.0.0.1"
	}
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz: %s", resp.Status)
	}
	return nil
}
