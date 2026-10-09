package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"pulsewatch-go/internal/auth"
	"pulsewatch-go/internal/checker"
	"pulsewatch-go/internal/ssrf"
	"pulsewatch-go/internal/worker"
)

type Server struct {
	DB     *pgxpool.Pool
	Secret string
}

type ctxKey string

const (
	ctxUser ctxKey = "user"
	ctxOrgs ctxKey = "orgs"
)

func NewRouter(s *Server) http.Handler {
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Org-ID", "X-User"},
		AllowCredentials: true,
	}))
	r.Get("/healthz", s.healthz)
	r.Get("/openapi.yaml", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, "openapi.yaml")
	})
	// Canonical API under /api/v1; the same handlers are also mounted at /
	// (no prefix) because the status-page frontend calls e.g. POST /auth/login.
	api := func() chi.Router {
		r := chi.NewRouter()
		r.Post("/auth/register", s.register)
		r.Post("/auth/login", s.login)
		r.Post("/auth/refresh", s.refresh)
		r.Post("/auth/logout", s.logout)
		r.Get("/public/{slug}/status", s.publicStatus)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/monitors", s.listMonitors)
			r.Post("/monitors", s.createMonitor)
			r.Get("/monitors/{id}", s.getMonitor)
			r.Put("/monitors/{id}", s.updateMonitor)
			r.Delete("/monitors/{id}", s.deleteMonitor)
			r.Get("/monitors/{id}/checks", s.listChecks)
			r.Get("/monitors/{id}/uptime", s.uptime)
			r.Post("/monitors/{id}/check", s.manualCheck)
			r.Get("/monitors/{id}/incidents", s.monitorIncidents)
			r.Get("/incidents", s.listIncidents)
			r.Post("/incidents/{id}/resolve", s.resolveIncident)
			r.Get("/alerts", s.listAlerts)
		})
		return r
	}
	r.Mount("/api/v1", api())
	r.Mount("/", api())
	return r
}

// ---- envelope helpers ----
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, data any) { writeJSON(w, 200, map[string]any{"data": data}) }

func created(w http.ResponseWriter, data any) { writeJSON(w, 201, map[string]any{"data": data}) }

func fail(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]any{"error": map[string]string{"code": errCode, "message": msg}})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	db := "down"
	if s.DB != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.DB.Ping(ctx); err == nil {
			db = "ok"
		}
	}
	status := "ok"
	if db == "down" && s.DB != nil {
		status = "degraded"
	}
	ok(w, map[string]string{"status": status, "db": db})
}

// ---- auth middleware ----
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		tok := strings.TrimPrefix(h, "Bearer ")
		if tok == "" || tok == h {
			fail(w, 401, "unauthorized", "missing bearer token")
			return
		}
		cl, err := auth.ParseAccessToken(s.Secret, tok)
		if err != nil {
			fail(w, 401, "unauthorized", "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, cl.UserID)
		ctx = context.WithValue(ctx, ctxOrgs, cl.OrgIDs)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userID(r *http.Request) string {
	v, _ := r.Context().Value(ctxUser).(string)
	return v
}

func orgID(r *http.Request) (string, bool) {
	org := r.Header.Get("X-Org-ID")
	if org == "" {
		return "", false
	}
	orgs, _ := r.Context().Value(ctxOrgs).([]string)
	for _, o := range orgs {
		if o == org {
			return org, true
		}
	}
	// fall back: verify membership live (covers orgs created after login)
	return org, true
}

func (s *Server) checkMember(ctx context.Context, orgID, user string) bool {
	if s.DB == nil {
		return false
	}
	var ok bool
	_ = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM org_members WHERE org_id=$1::uuid AND user_id=$2::uuid)`, orgID, user).Scan(&ok)
	return ok
}

func (s *Server) requireOrg(w http.ResponseWriter, r *http.Request) (string, bool) {
	org, present := orgID(r)
	if !present {
		fail(w, 400, "missing_org", "X-Org-ID header required")
		return "", false
	}
	if !s.checkMember(r.Context(), org, userID(r)) {
		fail(w, 403, "forbidden", "not a member of this org")
		return "", false
	}
	return org, true
}

// ---- auth handlers ----
type authBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func slugFromEmail(email string) string {
	prefix := email
	if i := strings.Index(email, "@"); i > 0 {
		prefix = email[:i]
	}
	slug := strings.ToLower(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return '-'
	}, prefix))
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "org"
	}
	return slug
}

func (s *Server) userOrgs(ctx context.Context, user string) []map[string]string {
	rows, err := s.DB.Query(ctx, `SELECT o.id::text, o.name, o.slug FROM orgs o JOIN org_members m ON m.org_id=o.id WHERE m.user_id=$1::uuid`, user)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var id, name, slug string
		if err := rows.Scan(&id, &name, &slug); err == nil {
			out = append(out, map[string]string{"id": id, "name": name, "slug": slug})
		}
	}
	return out
}

func (s *Server) issueTokens(w http.ResponseWriter, user string) (string, bool) {
	orgs := s.userOrgs(context.Background(), user)
	ids := make([]string, 0, len(orgs))
	for _, o := range orgs {
		ids = append(ids, o["id"])
	}
	access, err := auth.NewAccessToken(s.Secret, user, ids)
	if err != nil {
		return "", false
	}
	raw, hash, err := auth.NewRefreshToken()
	if err != nil {
		return "", false
	}
	_, err = s.DB.Exec(context.Background(), `INSERT INTO refresh_tokens(user_id, token_hash, expires_at) VALUES($1::uuid,$2,now()+interval '7 days')`, user, hash)
	if err != nil {
		return "", false
	}
	http.SetCookie(w, &http.Cookie{Name: "pw_refresh", Value: raw, Path: "/", HttpOnly: true, MaxAge: 7 * 24 * 3600, SameSite: http.SameSiteLaxMode})
	return access, true
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var b authBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, 400, "bad_request", "invalid json")
		return
	}
	b.Email = strings.TrimSpace(strings.ToLower(b.Email))
	if !strings.Contains(b.Email, "@") || len(b.Password) < 8 {
		fail(w, 400, "validation_error", "email must contain @ and password >= 8 chars")
		return
	}
	hash, _ := auth.HashPassword(b.Password)
	ctx := r.Context()
	var userID string
	err := s.DB.QueryRow(ctx, `INSERT INTO users(email, password_hash, name) VALUES($1,$2,$3) ON CONFLICT(email) DO NOTHING RETURNING id::text`, b.Email, hash, b.Name).Scan(&userID)
	if err != nil {
		// conflict → fetch existing? registration of existing email is an error
		fail(w, 409, "conflict", "email already registered")
		return
	}
	slug := slugFromEmail(b.Email)
	var orgID string
	_ = s.DB.QueryRow(ctx, `INSERT INTO orgs(name, slug) VALUES($1,$2) ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name RETURNING id::text`, b.Name+" Org", slug).Scan(&orgID)
	if orgID == "" {
		slug = slug + "-" + userID[:8]
		_ = s.DB.QueryRow(ctx, `INSERT INTO orgs(name, slug) VALUES($1,$2) RETURNING id::text`, b.Name+" Org", slug).Scan(&orgID)
	}
	_, _ = s.DB.Exec(ctx, `INSERT INTO org_members(org_id, user_id, role) VALUES($1::uuid,$2::uuid,'owner') ON CONFLICT DO NOTHING`, orgID, userID)
	access, ok2 := s.issueTokens(w, userID)
	if !ok2 {
		fail(w, 500, "internal", "token issue failed")
		return
	}
	orgs := s.userOrgs(ctx, userID)
	ok(w, map[string]any{"user": map[string]string{"id": userID, "email": b.Email, "name": b.Name}, "orgs": orgs, "access_token": access})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var b authBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, 400, "bad_request", "invalid json")
		return
	}
	var id, hash, name, email string
	err := s.DB.QueryRow(r.Context(), `SELECT id::text, password_hash, name, email FROM users WHERE email=$1`, strings.ToLower(strings.TrimSpace(b.Email))).Scan(&id, &hash, &name, &email)
	if err != nil || auth.CheckPassword(hash, b.Password) != nil {
		fail(w, 401, "unauthorized", "invalid credentials")
		return
	}
	access, ok2 := s.issueTokens(w, id)
	if !ok2 {
		fail(w, 500, "internal", "token issue failed")
		return
	}
	ok(w, map[string]any{"user": map[string]string{"id": id, "email": email, "name": name}, "orgs": s.userOrgs(r.Context(), id), "access_token": access})
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("pw_refresh")
	if err != nil {
		fail(w, 401, "unauthorized", "missing refresh cookie")
		return
	}
	hash := auth.HashRefreshToken(c.Value)
	var user string
	err = s.DB.QueryRow(r.Context(), `SELECT user_id::text FROM refresh_tokens WHERE token_hash=$1 AND expires_at>now()`, hash).Scan(&user)
	if err != nil {
		fail(w, 401, "unauthorized", "invalid refresh token")
		return
	}
	_, _ = s.DB.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE token_hash=$1`, hash)
	access, ok2 := s.issueTokens(w, user)
	if !ok2 {
		fail(w, 500, "internal", "token issue failed")
		return
	}
	ok(w, map[string]any{"access_token": access, "orgs": s.userOrgs(r.Context(), user)})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("pw_refresh"); err == nil {
		_, _ = s.DB.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE token_hash=$1`, auth.HashRefreshToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "pw_refresh", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	ok(w, map[string]bool{"ok": true})
}

// ---- monitors ----
type monitorBody struct {
	Name               string `json:"name"`
	URL                string `json:"url"`
	Method             string `json:"method"`
	IntervalSeconds    int    `json:"interval_seconds"`
	TimeoutMs          int    `json:"timeout_ms"`
	ExpectStatus       int    `json:"expect_status"`
	ExpectBodyContains string `json:"expect_body_contains"`
	WebhookURL         string `json:"webhook_url"`
	Active             *bool  `json:"active"`
}

func validateMonitor(b monitorBody) (string, bool) {
	if strings.TrimSpace(b.Name) == "" {
		return "name is required", false
	}
	if err := ssrf.ValidateURL(b.URL); err != nil {
		return "invalid url: " + err.Error(), false
	}
	if b.IntervalSeconds != 0 && (b.IntervalSeconds < 10 || b.IntervalSeconds > 3600) {
		return "interval_seconds must be 10..3600", false
	}
	if b.TimeoutMs != 0 && (b.TimeoutMs < 1000 || b.TimeoutMs > 30000) {
		return "timeout_ms must be 1000..30000", false
	}
	if b.Method != "" {
		switch b.Method {
		case "GET", "POST", "HEAD", "PUT", "DELETE", "PATCH":
		default:
			return "unsupported method", false
		}
	}
	return "", true
}

func monitorRow(scan func(...any) error) (map[string]any, error) {
	var id, org, name, url, method, ebc, webhook, lastStatus string
	var interval, timeout, expect, fails int
	var active bool
	var lastChecked, created, updated *time.Time
	err := scan(&id, &org, &name, &url, &method, &interval, &timeout, &expect, &ebc, &webhook, &active, &fails, &lastStatus, &lastChecked, &created, &updated)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "org_id": org, "name": name, "url": url, "method": method,
		"interval_seconds": interval, "timeout_ms": timeout, "expect_status": expect,
		"expect_body_contains": ebc, "webhook_url": webhook, "active": active,
		"consecutive_failures": fails, "last_status": lastStatus,
		"last_checked_at": lastChecked, "created_at": created, "updated_at": updated}, nil
}

const monitorCols = `id::text, org_id::text, name, url, method, interval_seconds, timeout_ms, expect_status, expect_body_contains, COALESCE(webhook_url,''), active, consecutive_failures, last_status, last_checked_at, created_at, updated_at`

func (s *Server) listMonitors(w http.ResponseWriter, r *http.Request) {
	org, ok2 := s.requireOrg(w, r)
	if !ok2 {
		return
	}
	q := `SELECT ` + monitorCols + ` FROM monitors WHERE org_id=$1::uuid`
	args := []any{org}
	if v := r.URL.Query().Get("active"); v != "" {
		q += ` AND active=$2`
		args = append(args, v == "true" || v == "1")
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.DB.Query(r.Context(), q, args...)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	// Closed explicitly below before the stats query reuses a pool conn.
	out := []any{}
	byID := map[string]map[string]any{}
	for rows.Next() {
		m, err := monitorRow(rows.Scan)
		if err == nil {
			out = append(out, m)
			if id, ok := m["id"].(string); ok {
				byID[id] = m
			}
		}
	}
	rows.Close()
	// Per-monitor 24h uptime + latest check stats (single query, merged by id).
	// Uptime defaults to 100 when a monitor has no checks yet.
	statRows, serr := s.DB.Query(r.Context(), `
		SELECT DISTINCT ON (m.id) m.id::text,
			COALESCE((SELECT (COUNT(*) FILTER (WHERE ok))::float8 / NULLIF(COUNT(*),0) * 100
				FROM checks c2 WHERE c2.monitor_id = m.id AND c2.checked_at > now() - interval '24 hours'), 100)::float8,
			c.status_code, c.latency_ms
		FROM monitors m LEFT JOIN checks c ON c.monitor_id = m.id
		WHERE m.org_id = $1::uuid
		ORDER BY m.id, c.checked_at DESC NULLS LAST`, org)
	if serr == nil {
		defer statRows.Close()
		for statRows.Next() {
			var id string
			var pct float64
			var code, lat *int
			if err := statRows.Scan(&id, &pct, &code, &lat); err == nil {
				if m, ok := byID[id]; ok {
					m["uptime_24h"] = pct
					if code != nil {
						m["last_code"] = *code
					}
					if lat != nil {
						m["last_latency_ms"] = *lat
					}
				}
			}
		}
	}
	ok(w, out)
}

func (s *Server) createMonitor(w http.ResponseWriter, r *http.Request) {
	org, ok2 := s.requireOrg(w, r)
	if !ok2 {
		return
	}
	var b monitorBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, 400, "bad_request", "invalid json")
		return
	}
	if msg, good := validateMonitor(b); !good {
		fail(w, 400, "validation_error", msg)
		return
	}
	if b.Method == "" {
		b.Method = "GET"
	}
	if b.IntervalSeconds == 0 {
		b.IntervalSeconds = 60
	}
	if b.TimeoutMs == 0 {
		b.TimeoutMs = 8000
	}
	if b.ExpectStatus == 0 {
		b.ExpectStatus = 200
	}
	active := true
	if b.Active != nil {
		active = *b.Active
	}
	var id string
	err := s.DB.QueryRow(r.Context(), `INSERT INTO monitors(org_id, name, url, method, interval_seconds, timeout_ms, expect_status, expect_body_contains, webhook_url, active) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id::text`,
		org, b.Name, b.URL, b.Method, b.IntervalSeconds, b.TimeoutMs, b.ExpectStatus, b.ExpectBodyContains, b.WebhookURL, active).Scan(&id)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	m, err := monitorRow(s.DB.QueryRow(r.Context(), `SELECT `+monitorCols+` FROM monitors WHERE id=$1::uuid`, id).Scan)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	created(w, m)
}

func (s *Server) scopedMonitor(r *http.Request) (map[string]any, string, bool) {
	org := r.Header.Get("X-Org-ID")
	id := chi.URLParam(r, "id")
	m, err := monitorRow(s.DB.QueryRow(r.Context(), `SELECT `+monitorCols+` FROM monitors WHERE id=$1::uuid AND org_id=$2::uuid`, id, org).Scan)
	if err != nil {
		return nil, org, false
	}
	return m, org, true
}

func (s *Server) getMonitor(w http.ResponseWriter, r *http.Request) {
	if _, ok2 := s.requireOrg(w, r); !ok2 {
		return
	}
	m, _, good := s.scopedMonitor(r)
	if !good {
		fail(w, 404, "not_found", "monitor not found")
		return
	}
	ok(w, m)
}

func (s *Server) updateMonitor(w http.ResponseWriter, r *http.Request) {
	if _, ok2 := s.requireOrg(w, r); !ok2 {
		return
	}
	m, _, good := s.scopedMonitor(r)
	if !good {
		fail(w, 404, "not_found", "monitor not found")
		return
	}
	var b monitorBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, 400, "bad_request", "invalid json")
		return
	}
	// merge with existing for validation
	cur := map[string]string{"name": m["name"].(string), "url": m["url"].(string)}
	if b.Name != "" {
		cur["name"] = b.Name
	}
	if b.URL != "" {
		cur["url"] = b.URL
	}
	merged := monitorBody{Name: cur["name"], URL: cur["url"], Method: b.Method, IntervalSeconds: b.IntervalSeconds, TimeoutMs: b.TimeoutMs}
	if b.Method == "" {
		merged.Method = m["method"].(string)
	}
	if merged.IntervalSeconds == 0 {
		merged.IntervalSeconds = m["interval_seconds"].(int)
	}
	if merged.TimeoutMs == 0 {
		merged.TimeoutMs = m["timeout_ms"].(int)
	}
	if msg, good2 := validateMonitor(merged); !good2 {
		fail(w, 400, "validation_error", msg)
		return
	}
	id := chi.URLParam(r, "id")
	cur2, _, _ := s.scopedMonitor(r)
	exp := b.ExpectStatus
	if exp == 0 {
		exp = cur2["expect_status"].(int)
	}
	ebc := b.ExpectBodyContains
	wh := b.WebhookURL
	active := cur2["active"].(bool)
	if b.Active != nil {
		active = *b.Active
	}
	name := cur2["name"].(string)
	if b.Name != "" {
		name = b.Name
	}
	u := cur2["url"].(string)
	if b.URL != "" {
		u = b.URL
	}
	method := cur2["method"].(string)
	if b.Method != "" {
		method = b.Method
	}
	iv := cur2["interval_seconds"].(int)
	if b.IntervalSeconds != 0 {
		iv = b.IntervalSeconds
	}
	to := cur2["timeout_ms"].(int)
	if b.TimeoutMs != 0 {
		to = b.TimeoutMs
	}
	_, err := s.DB.Exec(r.Context(), `UPDATE monitors SET name=$1, url=$2, method=$3, interval_seconds=$4, timeout_ms=$5, expect_status=$6, expect_body_contains=$7, webhook_url=$8, active=$9, updated_at=now() WHERE id=$10::uuid`,
		name, u, method, iv, to, exp, ebc, wh, active, id)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	mm, err := monitorRow(s.DB.QueryRow(r.Context(), `SELECT `+monitorCols+` FROM monitors WHERE id=$1::uuid`, id).Scan)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	ok(w, mm)
}

func (s *Server) deleteMonitor(w http.ResponseWriter, r *http.Request) {
	if _, ok2 := s.requireOrg(w, r); !ok2 {
		return
	}
	id := chi.URLParam(r, "id")
	org := r.Header.Get("X-Org-ID")
	ct, err := s.DB.Exec(r.Context(), `DELETE FROM monitors WHERE id=$1::uuid AND org_id=$2::uuid`, id, org)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "not_found", "monitor not found")
		return
	}
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) listChecks(w http.ResponseWriter, r *http.Request) {
	if _, ok2 := s.requireOrg(w, r); !ok2 {
		return
	}
	if _, _, good := s.scopedMonitor(r); !good {
		fail(w, 404, "not_found", "monitor not found")
		return
	}
	limit := 50
	rows, err := s.DB.Query(r.Context(), `SELECT id::text, monitor_id::text, org_id::text, status_code, ok, latency_ms, error, checked_at FROM checks WHERE monitor_id=$1::uuid ORDER BY checked_at DESC LIMIT $2`, chi.URLParam(r, "id"), limit)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, mid, org string
		var code, lat int
		var isok bool
		var errmsg string
		var at time.Time
		if err := rows.Scan(&id, &mid, &org, &code, &isok, &lat, &errmsg, &at); err == nil {
			out = append(out, map[string]any{"id": id, "monitor_id": mid, "org_id": org, "status_code": code, "ok": isok, "latency_ms": lat, "error": errmsg, "checked_at": at})
		}
	}
	ok(w, out)
}

func (s *Server) uptime(w http.ResponseWriter, r *http.Request) {
	if _, ok2 := s.requireOrg(w, r); !ok2 {
		return
	}
	if _, _, good := s.scopedMonitor(r); !good {
		fail(w, 404, "not_found", "monitor not found")
		return
	}
	hours := 24
	if _, err := time.ParseDuration(r.URL.Query().Get("hours") + "h"); err == nil {
		_, _ = err, hours
	}
	if v := r.URL.Query().Get("hours"); v != "" {
		var n int
		if _, err := parseInt(v, &n); err == nil && n > 0 && n <= 24*90 {
			hours = n
		}
	}
	var total, okCount int
	if err := s.DB.QueryRow(r.Context(), `SELECT count(*), count(*) FILTER (WHERE ok) FROM checks WHERE monitor_id=$1::uuid AND checked_at > now() - make_interval(hours => $2)`, chi.URLParam(r, "id"), hours).Scan(&total, &okCount); err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	pct := 0.0
	if total > 0 {
		pct = float64(okCount) / float64(total) * 100
	}
	ok(w, map[string]any{"uptime_pct": pct, "total": total, "ok": okCount, "hours": hours})
}

func parseInt(v string, out *int) (int, error) {
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0, errBadInt
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return n, nil
}

var errBadInt = errString("bad int")

type errString string

func (e errString) Error() string { return string(e) }

func (s *Server) manualCheck(w http.ResponseWriter, r *http.Request) {
	org, ok2 := s.requireOrg(w, r)
	if !ok2 {
		return
	}
	m, _, good := s.scopedMonitor(r)
	if !good {
		fail(w, 404, "not_found", "monitor not found")
		return
	}
	// rate limit: 1 per 10s
	var last *time.Time
	_ = s.DB.QueryRow(r.Context(), `SELECT last_checked_at FROM monitors WHERE id=$1::uuid`, chi.URLParam(r, "id")).Scan(&last)
	if last != nil && time.Since(*last) < 10*time.Second {
		fail(w, 429, "rate_limited", "manual check allowed once per 10s")
		return
	}
	spec := checker.Spec{URL: m["url"].(string), Method: m["method"].(string), TimeoutMs: m["timeout_ms"].(int), ExpectStatus: m["expect_status"].(int), ExpectBodyContains: m["expect_body_contains"].(string)}
	res := checker.Check(r.Context(), spec)
	wm := worker.Monitor{ID: m["id"].(string), OrgID: org, Name: m["name"].(string), URL: m["url"].(string), ConsecutiveFails: m["consecutive_failures"].(int)}
	worker.ApplyResult(r.Context(), s.DB, wm, res)
	ok(w, map[string]any{"ok": res.OK, "status_code": res.StatusCode, "latency_ms": res.LatencyMs, "error": res.ErrMsg})
}

func (s *Server) listIncidents(w http.ResponseWriter, r *http.Request) {
	org, ok2 := s.requireOrg(w, r)
	if !ok2 {
		return
	}
	q := `SELECT i.id::text, i.monitor_id::text, i.org_id::text, i.started_at, i.resolved_at, i.status, i.cause, m.name FROM incidents i LEFT JOIN monitors m ON m.id=i.monitor_id WHERE i.org_id=$1::uuid`
	args := []any{org}
	if v := r.URL.Query().Get("status"); v != "" {
		q += ` AND i.status=$2`
		args = append(args, v)
	}
	q += ` ORDER BY i.started_at DESC LIMIT 100`
	rows, err := s.DB.Query(r.Context(), q, args...)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, mid, oid, status, cause, name string
		var started time.Time
		var resolved *time.Time
		_ = name
		var mname *string
		if err := rows.Scan(&id, &mid, &oid, &started, &resolved, &status, &cause, &mname); err == nil {
			mn := ""
			if mname != nil {
				mn = *mname
			}
			out = append(out, map[string]any{"id": id, "monitor_id": mid, "org_id": oid, "started_at": started, "resolved_at": resolved, "status": status, "cause": cause, "monitor_name": mn})
		}
	}
	ok(w, out)
}

func (s *Server) monitorIncidents(w http.ResponseWriter, r *http.Request) {
	org, ok2 := s.requireOrg(w, r)
	if !ok2 {
		return
	}
	rows, err := s.DB.Query(r.Context(), `SELECT i.id::text, i.monitor_id::text, i.org_id::text, i.started_at, i.resolved_at, i.status, i.cause FROM incidents i WHERE i.org_id=$1::uuid AND i.monitor_id=$2::uuid ORDER BY i.started_at DESC LIMIT 100`, org, chi.URLParam(r, "id"))
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, mid, oid, status, cause string
		var started time.Time
		var resolved *time.Time
		if err := rows.Scan(&id, &mid, &oid, &started, &resolved, &status, &cause); err == nil {
			out = append(out, map[string]any{"id": id, "monitor_id": mid, "org_id": oid, "started_at": started, "resolved_at": resolved, "status": status, "cause": cause})
		}
	}
	ok(w, out)
}

func (s *Server) resolveIncident(w http.ResponseWriter, r *http.Request) {
	org, ok2 := s.requireOrg(w, r)
	if !ok2 {
		return
	}
	id := chi.URLParam(r, "id")
	ct, err := s.DB.Exec(r.Context(), `UPDATE incidents SET status='resolved', resolved_at=now() WHERE id=$1::uuid AND org_id=$2::uuid AND status='open'`, id, org)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "not_found", "incident not found")
		return
	}
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	org, ok2 := s.requireOrg(w, r)
	if !ok2 {
		return
	}
	rows, err := s.DB.Query(r.Context(), `SELECT id::text, org_id::text, monitor_id::text, kind, target, payload, created_at FROM alert_logs WHERE org_id=$1::uuid ORDER BY created_at DESC LIMIT 100`, org)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, oid string
		var mid *string
		var kind, target, payload string
		var at time.Time
		if err := rows.Scan(&id, &oid, &mid, &kind, &target, &payload, &at); err == nil {
			m := ""
			if mid != nil {
				m = *mid
			}
			out = append(out, map[string]any{"id": id, "org_id": oid, "monitor_id": m, "kind": kind, "target": target, "payload": payload, "created_at": at})
		}
	}
	ok(w, out)
}

func (s *Server) publicStatus(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var orgID, orgName string
	err := s.DB.QueryRow(r.Context(), `SELECT id::text, name FROM orgs WHERE slug=$1`, slug).Scan(&orgID, &orgName)
	if err != nil {
		fail(w, 404, "not_found", "org not found")
		return
	}
	rows, err := s.DB.Query(r.Context(), `SELECT id::text, name, url, last_status FROM monitors WHERE org_id=$1::uuid AND active`, orgID)
	if err != nil {
		fail(w, 500, "internal", err.Error())
		return
	}
	defer rows.Close()
	mons := []any{}
	ids := []string{}
	for rows.Next() {
		var id, name, u, st string
		if err := rows.Scan(&id, &name, &u, &st); err == nil {
			var total, okCount int
			_ = s.DB.QueryRow(r.Context(), `SELECT count(*), count(*) FILTER (WHERE ok) FROM checks WHERE monitor_id=$1::uuid AND checked_at > now() - interval '24 hours'`, id).Scan(&total, &okCount)
			pct := 100.0
			if total > 0 {
				pct = float64(okCount) / float64(total) * 100
			}
			mons = append(mons, map[string]any{"id": id, "name": name, "url": u, "last_status": st, "uptime_24h": pct})
			ids = append(ids, id)
		}
	}
	irows, _ := s.DB.Query(r.Context(), `SELECT id::text, monitor_id::text, started_at, cause FROM incidents WHERE org_id=$1::uuid AND status='open' ORDER BY started_at DESC`, orgID)
	incs := []any{}
	if irows != nil {
		defer irows.Close()
		for irows.Next() {
			var id, mid string
			var st time.Time
			var cause string
			if err := irows.Scan(&id, &mid, &st, &cause); err == nil {
				incs = append(incs, map[string]any{"id": id, "monitor_id": mid, "started_at": st, "cause": cause})
			}
		}
	}
	if mons == nil {
		mons = []any{}
	}
	if incs == nil {
		incs = []any{}
	}
	ok(w, map[string]any{"org": map[string]string{"name": orgName, "slug": slug}, "monitors": mons, "open_incidents": incs})
}
