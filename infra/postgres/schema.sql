-- The event_store is the source of truth for aggregate state.
-- Events are appended here and are never changed.
CREATE TABLE IF NOT EXISTS event_store (
    event_id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    aggregate_type VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    version INT NOT NULL,
    ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Enforce ordering and optimistic concurrency control
    CONSTRAINT uq_aggregate_version UNIQUE (aggregate_id, version)
);

-- Index for efficiently loading an aggregate's event stream.
CREATE INDEX IF NOT EXISTS idx_event_store_aggregate_id ON event_store (aggregate_id);

-- The snapshots table stores periodic snapshots of an aggregate's state
-- to optimize loading time for aggregates with long event histories.
CREATE TABLE IF NOT EXISTS snapshots (
    snapshot_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id UUID NOT NULL,
    aggregate_type VARCHAR(255) NOT NULL,
    aggregate_version INT NOT NULL,
    payload JSONB NOT NULL,
    ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Only one snapshot per aggregate version
    CONSTRAINT uq_snapshot_aggregate_version UNIQUE (aggregate_id, aggregate_version)
);

-- Index to quickly find the latest snapshot for an aggregate.
CREATE INDEX IF NOT EXISTS idx_snapshots_latest ON snapshots (aggregate_id, aggregate_version DESC);


-- The outbox table is a temporary queue for reliably publishing events
-- to external systems (like a message broker) after they have been
-- successfully committed to the event_store.
CREATE TABLE IF NOT EXISTS outbox (
    event_id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    aggregate_type VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    version INT NOT NULL,
    ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published BOOLEAN NOT NULL DEFAULT FALSE
);

-- Index for the relay to efficiently find unpublished events.
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox (ts) WHERE published = FALSE;

-- The processed_events table ensures that subscribers process each
-- event exactly once, making consumers idempotent.
CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID NOT NULL,
    subscriber_id VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, subscriber_id)
);
