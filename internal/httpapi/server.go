package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/auth"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/render"
	"github.com/beanruntime/bean/internal/uiassets"
	"github.com/beanruntime/bean/internal/uid"
	"github.com/beanruntime/bean/internal/view"
	"github.com/beanruntime/bean/internal/webform"
)

type Server struct {
	Kernel        *kernel.Kernel
	Store         *release.Store
	Auth          auth.Service
	Actions       action.Service
	Views         view.Service
	SecureCookies bool
	Logger        *slog.Logger
	limiter       *loginLimiter
}

func (s *Server) Handler() http.Handler {
	s.limiter = &loginLimiter{attempts: map[string][]time.Time{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /openapi.json", s.openapi)
	mux.HandleFunc("GET /docs", s.docs)
	mux.HandleFunc("GET /api/system/manifest", s.manifest)
	mux.HandleFunc("GET /api/system/session", s.session)
	mux.HandleFunc("GET /api/system/page", s.page)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/views/{name}", s.view)
	mux.HandleFunc("POST /api/actions/{name}", s.action)
	mux.HandleFunc("POST /api/webforms/{name}/submit", s.form)
	mux.HandleFunc("GET /api/admin/definitions", s.definitions)
	mux.HandleFunc("POST /api/admin/definitions", s.saveDefinition)
	mux.HandleFunc("PUT /api/admin/definitions/{id}", s.saveDefinition)
	mux.HandleFunc("POST /api/admin/releases/validate", s.validate)
	mux.HandleFunc("POST /api/admin/releases/publish", s.publish)
	mux.HandleFunc("GET /api/admin/releases", s.releases)
	mux.HandleFunc("GET /api/admin/audit", s.audit)
	mux.HandleFunc("/", s.fallback)
	return s.logging(s.requestID(mux))
}
func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	if _, ok := s.Kernel.Active(); !ok {
		problem(w, 503, "not_ready", "No active release.", "")
		return
	}
	write(w, 200, map[string]string{"status": "ready"})
}
func (s *Server) openapi(w http.ResponseWriter, _ *http.Request) {
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(a.OpenAPI)
}
func (s *Server) docs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, `<!doctype html><title>Bean API</title><main><h1>Bean API</h1><p>OpenAPI 3.1 is available at <a href="/openapi.json">/openapi.json</a>.</p><pre id="spec"></pre><script>fetch('/openapi.json').then(r=>r.json()).then(x=>spec.textContent=JSON.stringify(x,null,2))</script></main>`)
}
func (s *Server) manifest(w http.ResponseWriter, _ *http.Request) {
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", "")
		return
	}
	write(w, 200, map[string]any{"appId": a.AppID, "releaseId": a.ReleaseID, "version": a.Version, "entities": a.Entities, "views": a.Views, "actions": a.Actions, "webforms": a.Webforms, "pages": a.Pages})
}
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	c, session, ok := s.requestContext(r)
	if !ok {
		write(w, 200, map[string]any{"authenticated": false})
		return
	}
	write(w, 200, map[string]any{"authenticated": true, "user": c.User, "tenantId": c.TenantID, "csrfToken": session.CSRF})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.limiter.allow(ip) {
		problem(w, 429, "rate_limited", "Too many login attempts.", requestID(r))
		return
	}
	var in struct{ Email, Password string }
	if !decode(w, r, &in) {
		return
	}
	session, e := s.Auth.Login(r.Context(), in.Email, in.Password)
	if e != nil {
		problem(w, 401, "invalid_credentials", "Invalid email or password.", requestID(r))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "bean_session", Value: session.ID, Path: "/", Expires: session.Expires, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.SecureCookies})
	write(w, 200, map[string]any{"user": session.User, "csrfToken": session.CSRF})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	_, session, ok := s.requestContext(r)
	if ok {
		if !csrf(r, session.CSRF) {
			problem(w, 403, "csrf", "CSRF validation failed.", requestID(r))
			return
		}
		_ = s.Auth.Logout(r.Context(), session.ID)
	}
	http.SetCookie(w, &http.Cookie{Name: "bean_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.SecureCookies})
	write(w, 200, map[string]bool{"ok": true})
}
func (s *Server) view(w http.ResponseWriter, r *http.Request) {
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	rows, e := s.Views.Run(r.Context(), a, r.PathValue("name"), view.Params{Filter: queryMap(r), Limit: limit, Offset: offset}, s.ctx(r))
	if e != nil {
		respondError(w, r, e)
		return
	}
	b, e := render.JSON(rows)
	if e != nil {
		respondError(w, r, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	c, session, authed := s.requestContext(r)
	if authed && !csrf(r, session.CSRF) {
		problem(w, 403, "csrf", "CSRF validation failed.", requestID(r))
		return
	}
	var in map[string]any
	if !decode(w, r, &in) {
		return
	}
	out, e := s.Actions.Execute(r.Context(), a, r.PathValue("name"), in, c)
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, map[string]any{"data": out})
}
func (s *Server) form(w http.ResponseWriter, r *http.Request) {
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	f, ok := a.Webforms[r.PathValue("name")]
	if !ok {
		problem(w, 404, "not_found", "Webform not found.", requestID(r))
		return
	}
	var in map[string]any
	if !decode(w, r, &in) {
		return
	}
	c := s.ctx(r)
	if e := webform.Validate(f, in, c); e != nil {
		problem(w, 400, "validation", e.Error(), requestID(r))
		return
	}
	out, e := s.Actions.Execute(r.Context(), a, f.Action, in, c)
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, map[string]any{"data": out, "confirmation": f.Confirmation})
}
func (s *Server) definitions(w http.ResponseWriter, r *http.Request) {
	if !s.admin(w, r) {
		return
	}
	defs, e := s.Store.Draft(r.Context(), "default")
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, defs)
}
func (s *Server) saveDefinition(w http.ResponseWriter, r *http.Request) {
	if !s.adminMutation(w, r) {
		return
	}
	var d definition.Definition
	if !decode(w, r, &d) {
		return
	}
	if e := s.Store.SaveDefinition(r.Context(), "default", d); e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
func (s *Server) validate(w http.ResponseWriter, r *http.Request) {
	if !s.adminMutation(w, r) {
		return
	}
	result, e := s.Store.Validate(r.Context(), "default")
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, map[string]any{"valid": len(result.Diagnostics) == 0, "diagnostics": result.Diagnostics, "schema": result.Schema})
}
func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	if !s.adminMutation(w, r) {
		return
	}
	published, diagnostics, e := s.Store.Publish(r.Context(), "default")
	if e != nil {
		respondError(w, r, e)
		return
	}
	if len(diagnostics) > 0 {
		write(w, 400, map[string]any{"valid": false, "diagnostics": diagnostics})
		return
	}
	write(w, 200, published)
}
func (s *Server) releases(w http.ResponseWriter, r *http.Request) {
	if !s.admin(w, r) {
		return
	}
	rows, e := s.Store.Releases(r.Context(), "default")
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, rows)
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	if !s.admin(w, r) {
		return
	}
	rows, e := s.Actions.DB.Select(r.Context(), dbal.Select{Table: "bean_audit", OrderBy: []dbal.Order{{Column: "at", Desc: true}}, Limit: 200})
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, rows)
}
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	p, params, ok := page.Match(a, r.URL.Query().Get("path"))
	if !ok {
		problem(w, 404, "not_found", "Page not found.", requestID(r))
		return
	}
	if p.Policy != "" {
		definition, exists := a.Policies[p.Policy]
		if !exists || !policy.Can(definition, false, s.ctx(r), nil) {
			problem(w, 404, "not_found", "Page not found.", requestID(r))
			return
		}
	}
	ctx := map[string]any{}
	for k, v := range params {
		ctx[k] = v
	}
	write(w, 200, map[string]any{"tree": page.Node(a, p, ctx)})
}
func (s *Server) fallback(w http.ResponseWriter, r *http.Request) {
	if a, ok := s.Kernel.Active(); ok {
		if name, display, found := view.Display(a, r.URL.Path); found {
			rows, e := s.Views.Run(r.Context(), a, name, view.Params{}, s.ctx(r))
			if e != nil {
				respondError(w, r, e)
				return
			}
			switch display.Type {
			case "json":
				b, _ := render.JSON(rows)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(b)
				return
			case "csv":
				b, _ := render.CSV(rows)
				w.Header().Set("Content-Type", "text/csv")
				_, _ = w.Write(b)
				return
			case "rss":
				b, _ := render.RSS(name, absoluteURL(r), rows)
				w.Header().Set("Content-Type", "application/rss+xml")
				_, _ = w.Write(b)
				return
			}
		}
	}
	assets, e := uiassets.FS()
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	file := strings.TrimPrefix(r.URL.Path, "/")
	if file == "" || !strings.Contains(file, ".") {
		file = "index.html"
	}
	if _, e = assets.Open(file); e != nil {
		file = "index.html"
	}
	http.ServeFileFS(w, r, assets, file)
}
func (s *Server) admin(w http.ResponseWriter, r *http.Request) bool {
	c, _, ok := s.requestContext(r)
	if !ok || c.User == nil || !role(c.User.Roles, "administrator") {
		problem(w, 403, "forbidden", "Administrator access is required.", requestID(r))
		return false
	}
	return true
}
func (s *Server) adminMutation(w http.ResponseWriter, r *http.Request) bool {
	if !s.admin(w, r) {
		return false
	}
	_, session, _ := s.requestContext(r)
	if !csrf(r, session.CSRF) {
		problem(w, 403, "csrf", "CSRF validation failed.", requestID(r))
		return false
	}
	return true
}
func (s *Server) ctx(r *http.Request) beanctx.Request { c, _, _ := s.requestContext(r); return c }
func (s *Server) requestContext(r *http.Request) (beanctx.Request, auth.Session, bool) {
	c := beanctx.Request{RequestID: requestID(r), Route: r.URL.Path, RouteParams: map[string]string{}, Values: map[string]any{}}
	cookie, e := r.Cookie("bean_session")
	if e != nil {
		return c, auth.Session{}, false
	}
	session, e := s.Auth.Current(r.Context(), cookie.Value)
	if e != nil {
		return c, auth.Session{}, false
	}
	c.User = &session.User
	c.TenantID = session.TenantID
	return c, session, true
}
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uid.New()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}
func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		logger := s.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Info("request", "request_id", requestID(r), "method", r.Method, "route", r.URL.Path, "status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

type requestIDKey struct{}

func requestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey{}).(string)
	return id
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if e := dec.Decode(out); e != nil {
		problem(w, 400, "invalid_json", "Request body is invalid.", requestID(r))
		return false
	}
	return true
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, message, id string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "requestId": id}})
}
func respondError(w http.ResponseWriter, r *http.Request, e error) {
	status, code := 500, "internal"
	if x, ok := e.(*dbal.Error); ok {
		code = string(x.Code)
		switch x.Code {
		case dbal.NotFound:
			status = 404
		case dbal.InvalidQuery:
			status = 400
		case dbal.Conflict, dbal.UniqueViolation, dbal.ForeignKeyViolation:
			status = 409
		case dbal.Busy:
			status = 503
		}
		problem(w, status, code, x.Message, requestID(r))
		return
	}
	problem(w, status, code, "Internal server error.", requestID(r))
}
func csrf(r *http.Request, want string) bool {
	return want != "" && r.Header.Get("X-CSRF-Token") == want
}
func role(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}
func queryMap(r *http.Request) map[string]any {
	m := map[string]any{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			m[k] = v[0]
		}
	}
	return m
}
func absoluteURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + r.URL.Path
}
func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		return strings.TrimSpace(strings.Split(x, ",")[0])
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := time.Now().Add(-time.Minute)
	xs := l.attempts[ip][:0]
	for _, t := range l.attempts[ip] {
		if t.After(cut) {
			xs = append(xs, t)
		}
	}
	if len(xs) >= 10 {
		l.attempts[ip] = xs
		return false
	}
	l.attempts[ip] = append(xs, time.Now())
	return true
}

var _ = fmt.Sprint
