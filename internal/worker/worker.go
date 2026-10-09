package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pulsewatch-go/internal/alerts"
	"pulsewatch-go/internal/checker"
)

// Monitor is the minimal row the scheduler needs.
type Monitor struct {
	ID                 string
	OrgID              string
	Name               string
	URL                string
	Method             string
	IntervalSeconds    int
	TimeoutMs          int
	ExpectStatus       int
	ExpectBodyContains string
	WebhookURL         string
	ConsecutiveFails   int
}

// CheckFunc allows tests to fake the checker.
type CheckFunc func(ctx context.Context, m Monitor) checker.Result

// EvaluateCheck computes next failure count + status from a result.
func EvaluateCheck(m Monitor, ok bool) (failures int, status string) {
	if ok {
		return 0, "up"
	}
	return m.ConsecutiveFails + 1, "down"
}

// Pool runs the scheduler + workers.
type Pool struct {
	db    *pgxpool.Pool
	check CheckFunc
	wg    sync.WaitGroup
}

func New(db *pgxpool.Pool) *Pool {
	return &Pool{db: db, check: func(ctx context.Context, m Monitor) checker.Result {
		return checker.Check(ctx, checker.Spec{
			URL: s(m.URL), Method: m.Method, TimeoutMs: m.TimeoutMs,
			ExpectStatus: m.ExpectStatus, ExpectBodyContains: m.ExpectBodyContains,
		})
	}}
}

func s(v string) string { return v }

// Run ticks every 5s until ctx cancelled.
func (p *Pool) Run(ctx context.Context) {
	if p.db == nil {
		return
	}
	jobs := make(chan Monitor, 64)
	for i := 0; i < 8; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case m, ok := <-jobs:
					if !ok {
						return
					}
					p.runOne(ctx, m)
				}
			}
		}()
	}
	t := time.NewTicker(5 * time.Second)
	defer func() { t.Stop(); close(jobs); p.wg.Wait() }()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, m := range p.due(ctx) {
				select {
				case jobs <- m:
				case <-ctx.Done():
					return
				default:
					slog.Warn("worker queue full, skipping")
				}
			}
		}
	}
}

func (p *Pool) due(ctx context.Context) []Monitor {
	rows, err := p.db.Query(ctx, `SELECT id::text, org_id::text, name, url, method, interval_seconds, timeout_ms, expect_status, expect_body_contains, COALESCE(webhook_url,''), consecutive_failures FROM monitors WHERE active AND (last_checked_at IS NULL OR last_checked_at + (interval_seconds || ' seconds')::interval < now()) LIMIT 100`)
	if err != nil {
		slog.Error("scheduler query", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Monitor
	for rows.Next() {
		var m Monitor
		if err := rows.Scan(&m.ID, &m.OrgID, &m.Name, &m.URL, &m.Method, &m.IntervalSeconds, &m.TimeoutMs, &m.ExpectStatus, &m.ExpectBodyContains, &m.WebhookURL, &m.ConsecutiveFails); err == nil {
			out = append(out, m)
		}
	}
	return out
}

func (p *Pool) runOne(ctx context.Context, m Monitor) {
	res := p.check(ctx, m)
	ApplyResult(ctx, p.db, m, res)
	if m.WebhookURL != "" {
		msg := alerts.SendWebhook(ctx, m.WebhookURL, map[string]any{
			"monitor": m.Name, "ok": res.OK, "status_code": res.StatusCode, "latency_ms": res.LatencyMs,
		})
		payload := "delivered"
		target := m.WebhookURL
		if msg != "" {
			payload = msg
		}
		_, _ = p.db.Exec(ctx, `INSERT INTO alert_logs(org_id, monitor_id, kind, target, payload) VALUES($1,$2,'webhook',$3,$4)`, m.OrgID, m.ID, target, payload)
	}
}

// ApplyResult persists a check result: checks row, monitor counters, incident open/resolve.
func ApplyResult(ctx context.Context, db *pgxpool.Pool, m Monitor, res checker.Result) {
	failures, status := EvaluateCheck(m, res.OK)
	_, err := db.Exec(ctx, `INSERT INTO checks(monitor_id, org_id, status_code, ok, latency_ms, error) VALUES($1,$2,$3,$4,$5,$6)`,
		m.ID, m.OrgID, res.StatusCode, res.OK, res.LatencyMs, res.ErrMsg)
	if err != nil {
		slog.Error("insert check", "err", err)
	}
	_, _ = db.Exec(ctx, `UPDATE monitors SET consecutive_failures=$1, last_status=$2, last_checked_at=now(), updated_at=now() WHERE id=$3`, failures, status, m.ID)
	if !res.OK {
		var open bool
		_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND status='open')`, m.ID).Scan(&open)
		if !open {
			_, _ = db.Exec(ctx, `INSERT INTO incidents(monitor_id, org_id, cause) VALUES($1,$2,$3)`, m.ID, m.OrgID, res.ErrMsg)
			_, _ = db.Exec(ctx, `INSERT INTO alert_logs(org_id, monitor_id, kind, payload) VALUES($1,$2,'log','incident opened')`, m.OrgID, m.ID)
		}
	} else {
		var id string
		err := db.QueryRow(ctx, `SELECT id::text FROM incidents WHERE monitor_id=$1 AND status='open' ORDER BY started_at DESC LIMIT 1`, m.ID).Scan(&id)
		if err == nil {
			_, _ = db.Exec(ctx, `UPDATE incidents SET status='resolved', resolved_at=now() WHERE id=$1::uuid`, id)
			_, _ = db.Exec(ctx, `INSERT INTO alert_logs(org_id, monitor_id, kind, payload) VALUES($1,$2,'log','incident resolved')`, m.OrgID, m.ID)
		}
	}
}
