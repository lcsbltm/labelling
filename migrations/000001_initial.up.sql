BEGIN;

CREATE TABLE printers (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    model TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    nickname VARCHAR(255) CHECK (nickname != ''),
    location VARCHAR(255) CHECK (location != ''),
    serial_number VARCHAR(255) CHECK (serial_number != ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE labels (
    id TEXT PRIMARY KEY CHECK (id != ''),
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    nickname VARCHAR(255) CHECK (nickname != ''),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE inventory_sync_state (
    resource_type TEXT PRIMARY KEY CHECK (resource_type IN ('printers', 'labels')),
    last_successful_sync_at TIMESTAMPTZ
);

INSERT INTO inventory_sync_state (resource_type)
VALUES ('printers'), ('labels');

CREATE TABLE print_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    printer_id BIGINT NOT NULL REFERENCES printers (id) ON DELETE RESTRICT,
    label_id TEXT NOT NULL REFERENCES labels (id) ON DELETE RESTRICT,
    printer_name TEXT NOT NULL,
    observed_label_version TEXT NOT NULL,
    requested_by TEXT NOT NULL CHECK (requested_by != ''),
    idempotency_key VARCHAR(128) NOT NULL CHECK (idempotency_key ~ '^[A-Za-z0-9._:-]+$'),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'submitted', 'failed', 'unknown')),
    processing_started_at TIMESTAMPTZ,
    last_error_code TEXT,
    last_error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT print_jobs_requested_by_idempotency_unique UNIQUE (requested_by, idempotency_key)
);

CREATE INDEX print_jobs_pending_idx ON print_jobs (created_at, id) WHERE status = 'pending';

COMMIT;
