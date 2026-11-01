-- Link codes: only the SHA-256 of the code is stored (docs/CONTRACTS.md §4).
CREATE TABLE link_codes (
    hash        TEXT PRIMARY KEY,
    app_user_id TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    redeemed_at TEXT
);
CREATE INDEX link_codes_expires_at ON link_codes (expires_at);

-- RevenueCat webhook events, deduplicated on the event id.
CREATE TABLE purchase_events (
    id            TEXT PRIMARY KEY,
    type          TEXT NOT NULL,
    app_user_id   TEXT NOT NULL,
    product_id    TEXT NOT NULL,
    store         TEXT NOT NULL,
    price         REAL NOT NULL DEFAULT 0,
    currency      TEXT NOT NULL DEFAULT '',
    environment   TEXT NOT NULL DEFAULT '',
    received_at   TEXT NOT NULL,
    notify_status TEXT,
    notified_at   TEXT,
    raw           TEXT NOT NULL
);
CREATE INDEX purchase_events_received_at ON purchase_events (received_at);
