CREATE TABLE IF NOT EXISTS monitors (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  method VARCHAR(10) NOT NULL DEFAULT 'GET',
  interval_seconds INT NOT NULL DEFAULT 60 CHECK (interval_seconds BETWEEN 10 AND 3600),
  timeout_ms INT NOT NULL DEFAULT 8000 CHECK (timeout_ms BETWEEN 1000 AND 30000),
  expect_status INT NOT NULL DEFAULT 200,
  expect_body_contains TEXT NOT NULL DEFAULT '',
  webhook_url TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT true,
  consecutive_failures INT NOT NULL DEFAULT 0,
  last_status TEXT NOT NULL DEFAULT 'unknown',
  last_checked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_monitors_org_active ON monitors(org_id, active);

CREATE TABLE IF NOT EXISTS checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
  org_id UUID NOT NULL,
  status_code INT NOT NULL DEFAULT 0,
  ok BOOLEAN NOT NULL,
  latency_ms INT NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_checks_monitor_time ON checks(monitor_id, checked_at DESC);

CREATE TABLE IF NOT EXISTS incidents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
  org_id UUID NOT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'open',
  cause TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_incidents_monitor_status ON incidents(monitor_id, status);

CREATE TABLE IF NOT EXISTS alert_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL,
  monitor_id UUID REFERENCES monitors(id) ON DELETE SET NULL,
  kind TEXT NOT NULL,
  target TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
