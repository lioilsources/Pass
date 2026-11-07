// Package httpapi is the HTTP surface of pass-api (docs/CONTRACTS.md §4):
// catalog, entitlement lookup, link codes and the RevenueCat webhook.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lioilsources/Pass/internal/catalog"
	"github.com/lioilsources/Pass/internal/linkcode"
	"github.com/lioilsources/Pass/internal/notify"
	"github.com/lioilsources/Pass/internal/revenuecat"
	"github.com/lioilsources/Pass/internal/store"
)

// Entitlements is what the server needs from RevenueCat; *revenuecat.Client satisfies it.
type Entitlements interface {
	HasEntitlement(ctx context.Context, appUserID, entitlement string) (bool, error)
	Invalidate(appUserID string)
}

// Config wires the server.
type Config struct {
	Catalog       *catalog.Catalog
	CatalogJSON   []byte // served verbatim on /pass/apps.json
	Store         *store.Store
	Entitlements  Entitlements
	Notifier      *notify.Notifier
	WebhookSecret string // RC_WEBHOOK_SECRET; empty disables the webhook (501)
	Logger        *slog.Logger
	Now           func() time.Time
}

// Server is an http.Handler.
type Server struct {
	cfg         Config
	mux         *http.ServeMux
	redeemLimit *limiter
	issueLimit  *limiter
}

// New builds the router.
func New(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	s := &Server{
		cfg:         cfg,
		mux:         http.NewServeMux(),
		redeemLimit: newLimiter(10, cfg.Now),
		issueLimit:  newLimiter(5, cfg.Now),
	}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /pass/apps.json", s.appsJSON)
	s.mux.HandleFunc("GET /v1/entitlements", s.entitlements)
	s.mux.HandleFunc("POST /v1/link-code", s.linkCode)
	s.mux.HandleFunc("POST /v1/redeem", s.redeem)
	s.mux.HandleFunc("POST /webhooks/revenuecat", s.webhook)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such route")
	})
	return s
}

// ServeHTTP applies recover + request log around the mux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := s.cfg.Now()
	rw := &statusWriter{ResponseWriter: w, status: 200}
	defer func() {
		if rec := recover(); rec != nil {
			s.cfg.Logger.Error("panic", "err", fmt.Sprint(rec), "path", r.URL.Path)
			writeError(rw, http.StatusInternalServerError, "internal", "internal error")
		}
		s.cfg.Logger.Info("request", "method", r.Method, "path", r.URL.Path, "status", rw.status,
			"ms", time.Since(start).Milliseconds(), "ip", clientIP(r))
	}()
	s.mux.ServeHTTP(rw, r)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// clientIP trusts X-Forwarded-For only when the direct peer is Caddy on the
// loopback or a private network; otherwise RemoteAddr is the client.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if net.ParseIP(first) != nil {
				return first
			}
		}
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot read body")
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body must be JSON: "+err.Error())
		return false
	}
	return true
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("ok"))
}

func (s *Server) appsJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(s.cfg.CatalogJSON)
}

// entitlementFailed writes the error response for a failed entitlement check
// and reports whether it did. An unknown subscriber is not a failure.
func (s *Server) entitlementFailed(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil, errors.Is(err, revenuecat.ErrNotFound):
		return false
	case errors.Is(err, revenuecat.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "not_configured", "entitlement checks are not configured yet")
	default:
		s.cfg.Logger.Error("revenuecat", "err", err)
		writeError(w, http.StatusBadGateway, "upstream", "entitlement check failed")
	}
	return true
}

func (s *Server) entitlements(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	if user == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "query parameter user is required")
		return
	}
	ok, err := s.cfg.Entitlements.HasEntitlement(r.Context(), user, catalog.Entitlement)
	if s.entitlementFailed(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"app_user_id":       user,
		catalog.Entitlement: ok,
		"source":            "revenuecat",
		"checked_at":        store.FormatTime(s.cfg.Now()),
	})
}

func (s *Server) linkCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AppUserID string `json:"app_user_id"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.AppUserID = strings.TrimSpace(in.AppUserID)
	if in.AppUserID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "app_user_id is required")
		return
	}
	if !s.issueLimit.allow("user:" + in.AppUserID) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many codes for this user, try again in a minute")
		return
	}
	ok, err := s.cfg.Entitlements.HasEntitlement(r.Context(), in.AppUserID, catalog.Entitlement)
	if s.entitlementFailed(w, err) {
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "no_pass", "this user has no active Ol1n Pass")
		return
	}
	code, exp, err := linkcode.Issue(s.cfg.Store, in.AppUserID, s.cfg.Now())
	if err != nil {
		s.cfg.Logger.Error("link code", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not issue a code")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"code":       code,
		"display":    linkcode.Display(code),
		"expires_at": store.FormatTime(exp),
	})
}

func (s *Server) redeem(w http.ResponseWriter, r *http.Request) {
	if !s.redeemLimit.allow("ip:" + clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in a minute")
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	user, err := linkcode.Redeem(s.cfg.Store, in.Code, s.cfg.Now())
	if err != nil {
		if errors.Is(err, linkcode.ErrInvalid) {
			writeError(w, http.StatusNotFound, "invalid_code", "unknown, expired or already used code")
			return
		}
		s.cfg.Logger.Error("redeem", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not redeem")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"app_user_id": user})
}

func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	if s.cfg.WebhookSecret == "" {
		writeError(w, http.StatusNotImplemented, "disabled", "RC_WEBHOOK_SECRET is not set")
		return
	}
	// RevenueCat sends the Authorization header exactly as configured in the
	// dashboard, so accept both the bare secret and a "Bearer <secret>" form.
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	auth = strings.TrimPrefix(auth, "Bearer ")
	if subtle.ConstantTimeCompare([]byte(auth), []byte(s.cfg.WebhookSecret)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "bad webhook authorization")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot read body")
		return
	}
	ev, err := revenuecat.ParseWebhook(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	now := s.cfg.Now()
	if ev.Type == "TEST" {
		s.cfg.Logger.Info("revenuecat test event", "id", ev.ID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "test"})
		return
	}
	err = s.cfg.Store.InsertPurchaseEvent(store.PurchaseEvent{
		ID: ev.ID, Type: ev.Type, AppUserID: ev.AppUserID, ProductID: ev.ProductID, Store: ev.Store,
		Price: ev.Price, Currency: ev.Currency, Environment: ev.Environment, ReceivedAt: now, Raw: string(body),
	})
	if errors.Is(err, store.ErrDuplicate) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
		return
	}
	if err != nil {
		s.cfg.Logger.Error("store event", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not store event")
		return
	}
	if ev.AppUserID != "" {
		s.cfg.Entitlements.Invalidate(ev.AppUserID)
	}
	if ev.OriginalAppUserID != "" {
		s.cfg.Entitlements.Invalidate(ev.OriginalAppUserID)
	}
	status := "skipped"
	if revenuecat.Notified[ev.Type] && s.cfg.Notifier != nil && s.cfg.Notifier.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.cfg.Notifier.Send(ctx, ev); err != nil {
			s.cfg.Logger.Error("notify", "event", ev.ID, "err", err)
			status = "failed"
		} else {
			status = "notified"
		}
	}
	if err := s.cfg.Store.MarkNotified(ev.ID, status, s.cfg.Now()); err != nil {
		s.cfg.Logger.Error("mark notified", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "notify": status})
}
