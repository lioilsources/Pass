// Package linkcode generates and verifies the short codes that carry a Pass
// from one device or platform to another (docs/CONTRACTS.md §4): 8 characters
// from an alphabet without 0/O/1/I, 15 minute TTL, single use, stored only as a
// SHA-256 hash.
package linkcode

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Alphabet omits the characters people confuse when reading a code aloud.
const Alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Length is the number of characters in a code.
const Length = 8

// TTL is how long a code can be redeemed after it was created.
const TTL = 15 * time.Minute

// ErrInvalid is returned for unknown, expired and already redeemed codes alike;
// a caller must not be able to tell them apart.
var ErrInvalid = errors.New("linkcode: invalid code")

// Generate returns a new random code (upper case, no separator).
func Generate() (string, error) {
	max := big.NewInt(int64(len(Alphabet)))
	b := make([]byte, Length)
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("linkcode: %w", err)
		}
		b[i] = Alphabet[n.Int64()]
	}
	return string(b), nil
}

// Normalize strips separators and whitespace and upper-cases the input, so
// "abcd-efgh" and "ABCD EFGH" both become "ABCDEFGH". It returns an error when
// the result is not a well-formed code.
func Normalize(input string) (string, error) {
	var sb strings.Builder
	for _, r := range strings.ToUpper(input) {
		switch r {
		case '-', ' ', '\t', '\n', '\r', '_':
			continue
		}
		sb.WriteRune(r)
	}
	code := sb.String()
	if len(code) != Length {
		return "", ErrInvalid
	}
	for i := 0; i < len(code); i++ {
		if !strings.ContainsRune(Alphabet, rune(code[i])) {
			return "", ErrInvalid
		}
	}
	return code, nil
}

// Display formats a normalized code for humans: ABCD-EFGH.
func Display(code string) string {
	if len(code) != Length {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// Hash is what gets stored: hex SHA-256 of the normalized code.
func Hash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// Store persists codes. Implemented by internal/store on SQLite; the
// interface keeps the HTTP layer testable without a database.
type Store interface {
	// InsertLinkCode records a new code hash for a user.
	InsertLinkCode(hash, appUserID string, createdAt, expiresAt time.Time) error
	// RedeemLinkCode marks a live, unredeemed code as redeemed and returns its
	// user. It must return ErrInvalid when the code is unknown, expired or
	// already redeemed.
	RedeemLinkCode(hash string, now time.Time) (appUserID string, err error)
}

// Issue creates a code for appUserID and stores its hash.
func Issue(s Store, appUserID string, now time.Time) (code string, expiresAt time.Time, err error) {
	code, err = Generate()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = now.Add(TTL)
	if err := s.InsertLinkCode(Hash(code), appUserID, now, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return code, expiresAt, nil
}

// Redeem normalizes the input and consumes the code.
func Redeem(s Store, input string, now time.Time) (string, error) {
	code, err := Normalize(input)
	if err != nil {
		return "", err
	}
	return s.RedeemLinkCode(Hash(code), now)
}
