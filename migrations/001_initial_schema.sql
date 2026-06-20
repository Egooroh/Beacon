-- +goose Up

CREATE TABLE projects (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT        NOT NULL,
    token_hash   TEXT        NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE issues (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID        NOT NULL REFERENCES projects(id),
    fingerprint   TEXT        NOT NULL,
    title         TEXT        NOT NULL,
    level         TEXT        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'open',
    events_count  BIGINT      NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at  TIMESTAMPTZ NOT NULL,
    last_alert_at TIMESTAMPTZ,
    UNIQUE (project_id, fingerprint)
);
CREATE INDEX idx_issues_project_lastseen ON issues (project_id, last_seen_at DESC);

CREATE TABLE events (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID        NOT NULL REFERENCES projects(id),
    issue_id     UUID        REFERENCES issues(id),
    fingerprint  TEXT,
    level        TEXT        NOT NULL,
    message      TEXT        NOT NULL,
    environment  TEXT,
    release      TEXT,
    payload      JSONB       NOT NULL,
    processed    BOOLEAN     NOT NULL DEFAULT false,
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Partial index for cheap unprocessed-event queue scan (FR-5).
CREATE INDEX idx_events_unprocessed ON events (received_at) WHERE processed = false;
CREATE INDEX idx_events_issue_time   ON events (issue_id, received_at DESC);

CREATE TABLE subscriptions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID        NOT NULL REFERENCES projects(id),
    platform    TEXT        NOT NULL,
    chat_id     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, platform, chat_id)
);

-- +goose Down

DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS issues;
DROP TABLE IF EXISTS projects;
