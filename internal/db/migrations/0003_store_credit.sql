-- +goose Up

-- Store credit ledger (ADR-0020): append-only signed entries per customer.
-- Issued by a return resolved as store_credit; redeemed as a payment method.

CREATE TABLE store_credit_ledger (
    id           INTEGER PRIMARY KEY,
    customer_id  INTEGER NOT NULL REFERENCES customers(id),
    delta_minor  INTEGER NOT NULL,
    reason       TEXT    NOT NULL CHECK (reason IN ('issued','redeemed')),
    order_id     INTEGER REFERENCES orders(id),
    return_id    INTEGER REFERENCES returns(id),
    note         TEXT,
    created_by   INTEGER REFERENCES approved_users(id),
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX idx_store_credit_customer ON store_credit_ledger(customer_id, created_at);

-- +goose Down

DROP TABLE IF EXISTS store_credit_ledger;
