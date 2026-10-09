package seed

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"pulsewatch-go/internal/auth"
)

// DB is the minimal interface (*pgxpool.Pool satisfies it).
type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Run is idempotent (ON CONFLICT DO NOTHING).
func Run(ctx context.Context, db DB) error {
	hash, _ := auth.HashPassword("demo1234")
	var userID string
	_ = db.QueryRow(ctx, `INSERT INTO users(email, password_hash, name) VALUES('admin@pulsewatch.demo',$1,'Demo Admin') ON CONFLICT(email) DO NOTHING RETURNING id::text`, hash).Scan(&userID)
	if userID == "" {
		_ = db.QueryRow(ctx, `SELECT id::text FROM users WHERE email='admin@pulsewatch.demo'`).Scan(&userID)
	}
	var orgID string
	_ = db.QueryRow(ctx, `INSERT INTO orgs(name, slug) VALUES('Demo Ops','demo') ON CONFLICT(slug) DO NOTHING RETURNING id::text`).Scan(&orgID)
	if orgID == "" {
		_ = db.QueryRow(ctx, `SELECT id::text FROM orgs WHERE slug='demo'`).Scan(&orgID)
	}
	_, _ = db.Exec(ctx, `INSERT INTO org_members(org_id, user_id, role) VALUES($1::uuid,$2::uuid,'owner') ON CONFLICT DO NOTHING`, orgID, userID)
	demos := []struct{ name, url string }{
		{"Example", "https://example.com"},
		{"Httpbin 200", "https://httpbin.org/status/200"},
		{"Httpbin Delay", "https://httpbin.org/delay/1"},
	}
	for _, d := range demos {
		// Existence check first: monitors has no unique constraint, so
		// ON CONFLICT would never fire and every boot would duplicate rows.
		var mid string
		_ = db.QueryRow(ctx, `SELECT id::text FROM monitors WHERE org_id=$1::uuid AND name=$2 AND url=$3 ORDER BY created_at LIMIT 1`, orgID, d.name, d.url).Scan(&mid)
		if mid == "" {
			_ = db.QueryRow(ctx, `INSERT INTO monitors(org_id, name, url) VALUES($1::uuid,$2,$3) RETURNING id::text`, orgID, d.name, d.url).Scan(&mid)
		}
		if mid != "" {
			_, _ = db.Exec(ctx, `INSERT INTO checks(monitor_id, org_id, status_code, ok, latency_ms) SELECT $1::uuid, $2::uuid, 200, true, 120 WHERE NOT EXISTS (SELECT 1 FROM checks WHERE monitor_id=$1::uuid)`, mid, orgID)
		}
	}
	var first string
	_ = db.QueryRow(ctx, `SELECT id::text FROM monitors WHERE org_id=$1::uuid ORDER BY created_at LIMIT 1`, orgID).Scan(&first)
	if first != "" {
		_, _ = db.Exec(ctx, `INSERT INTO incidents(monitor_id, org_id, cause) SELECT $1::uuid, $2::uuid, 'sample seed incident' WHERE NOT EXISTS (SELECT 1 FROM incidents WHERE monitor_id=$1::uuid AND status='open')`, first, orgID)
	}
	return nil
}
