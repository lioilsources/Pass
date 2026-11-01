// Package asc creates the Ol1n Pass products in App Store Connect
// (docs/CONTRACTS.md §2, §5): a non-consumable per app and a yearly
// subscription in the shared group, each with an en-US localization, a USD
// price and world-wide availability.
//
// The provisioning logic (provision.go) talks to the API interface, which has
// two implementations: the JSON:API client in this file and a planning stub
// that records what would be called, so `passctl asc products plan` runs
// without network and the two never drift apart.
package asc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultBaseURL is the production API.
const DefaultBaseURL = "https://api.appstoreconnect.apple.com"

// Client is a minimal JSON:API client authenticated with an App Store Connect
// API key (team key, role App Manager or Admin — IAP creation needs that).
type Client struct {
	KeyID    string
	IssuerID string
	Key      *ecdsa.PrivateKey
	BaseURL  string
	HTTP     *http.Client
	now      func() time.Time
}

// NewFromEnv reads ASC_KEY_ID, ASC_ISSUER_ID and ASC_PRIVATE_KEY_PATH (the
// same names PromoClown's store-reviews uses).
func NewFromEnv() (*Client, error) {
	keyID, issuer, path := os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_ISSUER_ID"), os.Getenv("ASC_PRIVATE_KEY_PATH")
	if keyID == "" || issuer == "" || path == "" {
		return nil, fmt.Errorf("asc: ASC_KEY_ID, ASC_ISSUER_ID and ASC_PRIVATE_KEY_PATH are required")
	}
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("asc: %w", err)
	}
	return New(keyID, issuer, pem)
}

// New builds a client from a .p8 PEM.
func New(keyID, issuerID string, pem []byte) (*Client, error) {
	key, err := jwt.ParseECPrivateKeyFromPEM(pem)
	if err != nil {
		return nil, fmt.Errorf("asc: private key: %w", err)
	}
	return &Client{
		KeyID: keyID, IssuerID: issuerID, Key: key,
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		now:     time.Now,
	}, nil
}

// Token mints a short-lived ES256 JWT (Apple allows at most 20 minutes).
func (c *Client) Token() (string, error) {
	now := c.now()
	claims := jwt.RegisteredClaims{
		Issuer:    c.IssuerID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		Audience:  jwt.ClaimStrings{"appstoreconnect-v1"},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = c.KeyID
	return tok.SignedString(c.Key)
}

// Resource is one JSON:API object.
type Resource struct {
	Type          string                     `json:"type"`
	ID            string                     `json:"id,omitempty"`
	Attributes    map[string]any             `json:"attributes,omitempty"`
	Relationships map[string]json.RawMessage `json:"relationships,omitempty"`
}

// Str returns a string attribute or "".
func (r Resource) Str(name string) string {
	v, _ := r.Attributes[name].(string)
	return v
}

// Document is a JSON:API response (single or list).
type Document struct {
	Data     json.RawMessage   `json:"data"`
	Included []Resource        `json:"included,omitempty"`
	Links    map[string]string `json:"links,omitempty"`
	Errors   []APIError        `json:"errors,omitempty"`
}

// APIError is one entry of an ASC error response.
type APIError struct {
	Status string `json:"status"`
	Code   string `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// One decodes a single-resource document.
func (d *Document) One() (Resource, error) {
	var r Resource
	if len(d.Data) == 0 || string(d.Data) == "null" {
		return r, errNotFound
	}
	return r, json.Unmarshal(d.Data, &r)
}

// Many decodes a list document.
func (d *Document) Many() ([]Resource, error) {
	var rs []Resource
	if len(d.Data) == 0 || string(d.Data) == "null" {
		return nil, nil
	}
	return rs, json.Unmarshal(d.Data, &rs)
}

var errNotFound = fmt.Errorf("asc: not found")

// HTTPError carries the status and the decoded ASC errors, so a wrong request
// shape is readable in the log.
type HTTPError struct {
	Method, Path string
	Status       int
	Errors       []APIError
	Body         string
}

func (e *HTTPError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "asc: %s %s → %d", e.Method, e.Path, e.Status)
	for _, ae := range e.Errors {
		fmt.Fprintf(&sb, "\n  [%s] %s: %s", ae.Code, ae.Title, ae.Detail)
	}
	if len(e.Errors) == 0 && e.Body != "" {
		fmt.Fprintf(&sb, "\n  %s", e.Body)
	}
	return sb.String()
}

// IsNotFound reports a 404 or an empty single resource.
func IsNotFound(err error) bool {
	if err == errNotFound {
		return true
	}
	if he, ok := err.(*HTTPError); ok {
		return he.Status == http.StatusNotFound
	}
	return false
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*Document, error) {
	tok, err := c.Token()
	if err != nil {
		return nil, err
	}
	u := strings.TrimRight(c.BaseURL, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("asc: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var doc Document
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &doc)
	}
	if resp.StatusCode/100 != 2 {
		return nil, &HTTPError{Method: method, Path: path, Status: resp.StatusCode, Errors: doc.Errors, Body: truncate(raw, 600)}
	}
	return &doc, nil
}

// Get performs GET and follows `links.next` so lists are complete.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (*Document, error) {
	doc, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	// Pagination: merge subsequent pages into Data when it is a list.
	for next := doc.Links["next"]; next != ""; {
		nu, err := url.Parse(next)
		if err != nil {
			break
		}
		page, err := c.do(ctx, http.MethodGet, nu.Path, nu.Query(), nil)
		if err != nil {
			return nil, err
		}
		a, _ := doc.Many()
		b, _ := page.Many()
		merged, _ := json.Marshal(append(a, b...))
		doc.Data = merged
		doc.Included = append(doc.Included, page.Included...)
		next = page.Links["next"]
	}
	return doc, nil
}

// Post performs POST with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (*Document, error) {
	return c.do(ctx, http.MethodPost, path, nil, body)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
