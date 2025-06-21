CREATE TABLE IF NOT EXISTS outbox (
    event_id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    version INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published BOOLEAN NOT NULL DEFAULT FALSE
);

-- Index for the relay to efficiently find unpublished events
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox (created_at) WHERE published = FALSE;


CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID NOT NULL,
    subscriber_id VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, subscriber_id)
);

-- A sample read model table to demonstrate versioning
CREATE TABLE IF NOT EXISTS product_views (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    version INT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
