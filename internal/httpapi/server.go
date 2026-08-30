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
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/audit"
	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/block"
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
	signupLimiter *loginLimiter
}

func (s *Server) Handler() http.Handler {
	s.limiter = &loginLimiter{attempts: map[string][]time.Time{}}
	s.signupLimiter = &loginLimiter{attempts: map[string][]time.Time{}}
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
	mux.HandleFunc("GET /api/admin/manifest", s.adminManifest)
	mux.HandleFunc("GET /api/admin/resources/{name}", s.adminResourceList)
	mux.HandleFunc("GET /api/admin/resources/{name}/{id}", s.adminResourceRecord)
	mux.HandleFunc("POST /api/admin/definitions", s.saveDefinition)
	mux.HandleFunc("PUT /api/admin/definitions/{id}", s.saveDefinition)
	mux.HandleFunc("POST /api/admin/releases/validate", s.validate)
	mux.HandleFunc("POST /api/admin/releases/publish", s.publish)
	mux.HandleFunc("GET /api/admin/releases", s.releases)
	mux.HandleFunc("GET /api/admin/audit", s.audit)
	mux.HandleFunc("GET /api/admin/system/summary", s.systemSummary)
	mux.HandleFunc("GET /api/admin/system/users", s.systemUsers)
	mux.HandleFunc("POST /api/admin/system/users", s.systemCreateUser)
	mux.HandleFunc("PUT /api/admin/system/users/{id}/roles", s.systemUpdateUser)
	mux.HandleFunc("GET /api/admin/system/jobs", s.systemJobs)
	mux.HandleFunc("POST /api/admin/system/jobs/{id}/{operation}", s.systemJobMutation)
	mux.HandleFunc("GET /api/admin/system/outbox", s.systemOutbox)
	mux.HandleFunc("POST /api/admin/system/outbox/{id}/{operation}", s.systemOutboxMutation)
	mux.HandleFunc("GET /api/admin/system/migrations", s.systemMigrations)
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
	write(w, 200, map[string]any{"appId": a.AppID, "releaseId": a.ReleaseID, "version": a.Version, "entities": a.Entities, "views": a.Views, "actions": a.Actions, "filters": a.Filters, "webforms": a.Webforms, "pages": a.Pages, "localRegistration": a.LocalRegistration})
}
func (s *Server) adminManifest(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	write(w, 200, map[string]any{"appId": a.AppID, "releaseId": a.ReleaseID, "version": a.Version, "entities": a.Entities, "views": a.Views, "actions": a.Actions, "adminResources": a.AdminResources, "systemAdmin": role(s.ctx(r).User.Roles, "administrator")})
}
func (s *Server) adminResourceList(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	resource, ok := a.AdminResources[r.PathValue("name")]
	if !ok {
		problem(w, 404, "not_found", "Admin resource not found.", requestID(r))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > resource.List.PageSize {
		limit = resource.List.PageSize
	}
	exact := map[string]any{}
	allowedFilters := stringSet(resource.List.Filters)
	for key := range r.URL.Query() {
		if !strings.HasPrefix(key, "filter.") {
			continue
		}
		field := strings.TrimPrefix(key, "filter.")
		if !allowedFilters[field] {
			problem(w, 400, "invalid_query", "Filter is not configured for this admin resource.", requestID(r))
			return
		}
		if value := r.URL.Query().Get(key); value != "" {
			exact[field] = value
		}
	}
	sortDefinitions := resource.List.Sort
	if field := r.URL.Query().Get("sort"); field != "" {
		if !stringSet(resource.List.Columns)[field] {
			problem(w, 400, "invalid_query", "Sort field is not configured for this admin resource.", requestID(r))
			return
		}
		sortDefinitions = []appir.Sort{{Field: field, Desc: r.URL.Query().Get("direction") == "desc"}}
	}
	result, e := s.Views.RunPage(r.Context(), a, resource.View, view.Params{ExactFilters: exact, Search: r.URL.Query().Get("q"), SearchFields: resource.List.Search, Sort: sortDefinitions, Limit: limit, Cursor: r.URL.Query().Get("cursor")}, s.ctx(r))
	if e != nil {
		respondError(w, r, e)
		return
	}
	if result.NextCursor != "" {
		w.Header().Set("Bean-Next-Cursor", result.NextCursor)
	}
	write(w, 200, map[string]any{"data": result.Rows, "nextCursor": result.NextCursor})
}
func (s *Server) adminResourceRecord(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	resource, ok := a.AdminResources[r.PathValue("name")]
	if !ok {
		problem(w, 404, "not_found", "Admin resource not found.", requestID(r))
		return
	}
	result, e := s.Views.RunPage(r.Context(), a, resource.View, view.Params{RecordID: r.PathValue("id"), Limit: 1}, s.ctx(r))
	if e != nil {
		respondError(w, r, e)
		return
	}
	if len(result.Rows) == 0 {
		problem(w, 404, "not_found", "Record not found.", requestID(r))
		return
	}
	write(w, 200, map[string]any{"data": result.Rows[0]})
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
	filters := queryMap(r)
	bound, e := s.boundBlockInputs(r, a, "view", r.PathValue("name"))
	if e != nil {
		problem(w, 400, "bound_input", e.Error(), requestID(r))
		return
	}
	for name, value := range bound {
		if _, collision := filters[name]; collision {
			problem(w, 400, "bound_input", "Bound input cannot be supplied by the client.", requestID(r))
			return
		}
		filters[name] = value
	}
	if blockName := r.URL.Query().Get("_block"); blockName != "" {
		block := a.Blocks[blockName]
		if block.Type == "resource-list" {
			allowed := stringSet(block.Filters)
			for name := range block.Bindings {
				allowed[name] = true
			}
			for name := range filters {
				if !allowed[name] {
					problem(w, 400, "invalid_query", "Filter is not configured for this resource list.", requestID(r))
					return
				}
			}
		}
	}
	result, e := s.Views.RunPage(r.Context(), a, r.PathValue("name"), view.Params{Filter: filters, Limit: limit, Offset: offset, Cursor: r.URL.Query().Get("cursor")}, s.ctx(r))
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, map[string]any{"data": result.Rows, "nextCursor": result.NextCursor})
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
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		in["_idempotencyKey"] = key
	}
	if definition := a.Actions[r.PathValue("name")]; definition.Operation == "register_local_user" && (a.LocalRegistration == nil || a.LocalRegistration.Action != definition.Name) {
		problem(w, 404, "not_found", "Action not found.", requestID(r))
		return
	} else if definition.Operation == "register_local_user" && !s.signupLimiter.allow(clientIP(r)) {
		problem(w, 429, "rate_limited", "Too many signup attempts.", requestID(r))
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
	actionDefinition := a.Actions[f.Action]
	if actionDefinition.Operation == "register_local_user" {
		if a.LocalRegistration == nil || a.LocalRegistration.Action != actionDefinition.Name {
			problem(w, 404, "not_found", "Webform not found.", requestID(r))
			return
		}
		if !s.signupLimiter.allow(clientIP(r)) {
			problem(w, 429, "rate_limited", "Too many signup attempts.", requestID(r))
			return
		}
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
	bound, e := s.boundBlockInputs(r, a, "webform", f.Name)
	if e != nil {
		problem(w, 400, "bound_input", e.Error(), requestID(r))
		return
	}
	for name, value := range bound {
		if _, collision := in[name]; collision {
			problem(w, 400, "bound_input", "Bound input cannot be supplied by the client.", requestID(r))
			return
		}
		in[name] = value
	}
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		in["_idempotencyKey"] = key
	}
	if e := webform.Validate(f, in, c); e != nil {
		if fields, ok := e.(webform.Errors); ok {
			write(w, 400, map[string]any{"error": map[string]any{"code": "validation", "message": e.Error(), "requestId": requestID(r), "fields": fields}})
		} else {
			problem(w, 400, "validation", e.Error(), requestID(r))
		}
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
	result, migrationPlan, e := s.Store.Preview(r.Context(), "default")
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, map[string]any{"valid": len(result.Diagnostics) == 0, "diagnostics": result.Diagnostics, "schema": result.Schema, "migration": migrationPlan})
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
	if !s.editor(w, r) {
		return
	}
	predicates := []dbal.Predicate{}
	for query, column := range map[string]string{"entity": "entity_type", "id": "entity_id"} {
		if value := r.URL.Query().Get(query); value != "" {
			predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: column, Value: value})
		}
	}
	var where *dbal.Predicate
	if len(predicates) == 1 {
		where = &predicates[0]
	} else if len(predicates) > 1 {
		combined := dbal.And(predicates...)
		where = &combined
	}
	rows, e := s.Actions.DB.Select(r.Context(), dbal.Select{Table: "bean_audit", Where: where, OrderBy: []dbal.Order{{Column: "at", Desc: true}}, Limit: 200})
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, rows)
}

func (s *Server) systemSummary(w http.ResponseWriter, r *http.Request) {
	if !s.admin(w, r) {
		return
	}
	app, active := s.Kernel.Active()
	jobs, err := s.Actions.DB.Select(r.Context(), dbal.Select{Table: "bean_job", Columns: []string{"status", "claimed_at"}, Limit: 10000})
	if err != nil {
		respondError(w, r, err)
		return
	}
	outbox, err := s.Actions.DB.Select(r.Context(), dbal.Select{Table: "bean_outbox", Columns: []string{"status", "claimed_at"}, Limit: 10000})
	if err != nil {
		respondError(w, r, err)
		return
	}
	releaseID, version := "", 0
	if active {
		releaseID, version = app.ReleaseID, app.Version
	}
	write(w, 200, map[string]any{"releaseId": releaseID, "version": version, "jobs": statusCounts(jobs), "outbox": statusCounts(outbox)})
}

func (s *Server) systemUsers(w http.ResponseWriter, r *http.Request) {
	if !s.admin(w, r) {
		return
	}
	rows, err := s.Actions.DB.Select(r.Context(), dbal.Select{Table: "bean_user", Columns: []string{"id", "email", "roles", "tenant_id", "created_at"}, OrderBy: []dbal.Order{{Column: "email"}}, Limit: 500})
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, rows)
}

func (s *Server) systemCreateUser(w http.ResponseWriter, r *http.Request) {
	if !s.adminMutation(w, r) {
		return
	}
	var input struct {
		Email    string   `json:"email"`
		Password string   `json:"password"`
		Roles    []string `json:"roles"`
		TenantID string   `json:"tenantId"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.Actions.DB.Transaction(r.Context(), func(tx dbal.Transaction) error {
		userID, created, createErr := s.Auth.CreateInTransaction(r.Context(), tx, input.Email, input.Password, input.Roles, input.TenantID)
		if createErr != nil || !created {
			return createErr
		}
		return audit.Write(r.Context(), tx, audit.Entry{RequestID: requestID(r), UserID: s.ctx(r).User.ID, Action: "system_user_create", EntityType: "bean_user", EntityID: userID, Changed: []string{"email", "roles", "tenant_id"}, Success: true})
	}); err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 201, map[string]bool{"ok": true})
}

func (s *Server) systemUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !s.adminMutation(w, r) {
		return
	}
	var input struct {
		Roles    []string `json:"roles"`
		TenantID string   `json:"tenantId"`
	}
	if !decode(w, r, &input) {
		return
	}
	roles, _ := json.Marshal(input.Roles)
	err := s.Actions.DB.Transaction(r.Context(), func(tx dbal.Transaction) error {
		if _, updateErr := tx.Update(r.Context(), dbal.Update{Table: "bean_user", Values: map[string]dbal.Value{"roles": string(roles), "tenant_id": nullable(input.TenantID)}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: r.PathValue("id")}, ExpectedRows: 1}); updateErr != nil {
			return updateErr
		}
		return audit.Write(r.Context(), tx, audit.Entry{RequestID: requestID(r), UserID: s.ctx(r).User.ID, Action: "system_user_roles_update", EntityType: "bean_user", EntityID: r.PathValue("id"), Changed: []string{"roles", "tenant_id"}, Success: true})
	})
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (s *Server) systemJobs(w http.ResponseWriter, r *http.Request) {
	s.systemRows(w, r, "bean_job", []string{"id", "name", "run_at", "status", "attempts", "max_attempts", "next_attempt_at", "claimed_at", "last_error", "completed_at"}, "run_at")
}

func (s *Server) systemOutbox(w http.ResponseWriter, r *http.Request) {
	s.systemRows(w, r, "bean_outbox", []string{"id", "topic", "created_at", "status", "attempts", "max_attempts", "next_attempt_at", "claimed_at", "last_error", "delivered_at"}, "created_at")
}

func (s *Server) systemMigrations(w http.ResponseWriter, r *http.Request) {
	s.systemRows(w, r, "bean_schema_migration", []string{"release_id", "sequence", "description", "applied_at"}, "applied_at")
}

func (s *Server) systemRows(w http.ResponseWriter, r *http.Request, table string, columns []string, order string) {
	if !s.admin(w, r) {
		return
	}
	rows, err := s.Actions.DB.Select(r.Context(), dbal.Select{Table: table, Columns: columns, OrderBy: []dbal.Order{{Column: order, Desc: true}}, Limit: 500})
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, rows)
}

func (s *Server) systemJobMutation(w http.ResponseWriter, r *http.Request) {
	s.systemQueueMutation(w, r, "bean_job")
}

func (s *Server) systemOutboxMutation(w http.ResponseWriter, r *http.Request) {
	s.systemQueueMutation(w, r, "bean_outbox")
}

func (s *Server) systemQueueMutation(w http.ResponseWriter, r *http.Request, table string) {
	if !s.adminMutation(w, r) {
		return
	}
	operation := r.PathValue("operation")
	if operation != "retry" && operation != "cancel" {
		problem(w, 404, "not_found", "System operation not found.", requestID(r))
		return
	}
	eligible := dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "failed"}
	values := map[string]dbal.Value{"claim_token": nil, "claimed_at": nil}
	if operation == "retry" {
		values["status"] = "pending"
		values["attempts"] = 0
		values["last_error"] = nil
		values["next_attempt_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	} else {
		eligible = dbal.Or(eligible, dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "pending"})
		values["status"] = "canceled"
	}
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: r.PathValue("id")}, eligible)
	err := s.Actions.DB.Transaction(r.Context(), func(tx dbal.Transaction) error {
		if _, updateErr := tx.Update(r.Context(), dbal.Update{Table: table, Values: values, Where: where, ExpectedRows: 1}); updateErr != nil {
			return updateErr
		}
		return audit.Write(r.Context(), tx, audit.Entry{RequestID: requestID(r), UserID: s.ctx(r).User.ID, Action: "system_" + strings.TrimPrefix(table, "bean_") + "_" + operation, EntityType: table, EntityID: r.PathValue("id"), Changed: []string{"status"}, Success: true})
	})
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func statusCounts(rows []dbal.Row) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[fmt.Sprint(row["status"])]++
	}
	return counts
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
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
	query := map[string]string{}
	for key := range r.URL.Query() {
		query[key] = r.URL.Query().Get(key)
	}
	ctx, e := page.ResolveContext(p, params, query, s.ctx(r))
	if e != nil {
		problem(w, 400, "missing_context", "Required page context is missing.", requestID(r))
		return
	}
	tree, allowed, e := page.Node(a, p, ctx, s.ctx(r))
	if e != nil {
		problem(w, 400, "missing_context", "Required render context is missing.", requestID(r))
		return
	}
	if !allowed {
		problem(w, 404, "not_found", "Page not found.", requestID(r))
		return
	}
	write(w, 200, map[string]any{"tree": tree})
}

func (s *Server) boundBlockInputs(r *http.Request, a *appir.App, kind, target string) (map[string]any, error) {
	pagePath, blockName := r.URL.Query().Get("_page"), r.URL.Query().Get("_block")
	if pagePath == "" && blockName == "" {
		return nil, nil
	}
	if pagePath == "" || blockName == "" {
		return nil, fmt.Errorf("page and block context must be supplied together")
	}
	p, routeParams, matched := page.Match(a, pagePath)
	if !matched {
		return nil, fmt.Errorf("bound page was not found")
	}
	found := false
	for _, region := range a.Panels[p.Panel].Regions {
		for _, candidate := range region.Blocks {
			found = found || candidate == blockName
		}
	}
	definition, exists := a.Blocks[blockName]
	if !found || !exists || kind == "view" && definition.View != target || kind == "webform" && definition.Webform != target {
		return nil, fmt.Errorf("bound block does not match this request")
	}
	query := map[string]string{}
	for key := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {
			query[key] = r.URL.Query().Get(key)
		}
	}
	contextValues, err := page.ResolveContext(p, routeParams, query, s.ctx(r))
	if err != nil {
		return nil, fmt.Errorf("required bound context is missing")
	}
	node, allowed, err := block.Node(a, definition, contextValues, s.ctx(r))
	if err != nil || !allowed {
		return nil, fmt.Errorf("bound block is unavailable")
	}
	inputs, _ := node.Props["inputs"].(map[string]any)
	return inputs, nil
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
func (s *Server) editor(w http.ResponseWriter, r *http.Request) bool {
	c, _, ok := s.requestContext(r)
	if !ok || c.User == nil || !role(c.User.Roles, "administrator") && !role(c.User.Roles, "editor") {
		problem(w, 403, "forbidden", "Editor access is required.", requestID(r))
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
	dec.DisallowUnknownFields()
	if e := dec.Decode(out); e != nil {
		problem(w, 400, "invalid_json", "Request body is invalid.", requestID(r))
		return false
	}
	var trailing any
	if e := dec.Decode(&trailing); e != io.EOF {
		problem(w, 400, "invalid_json", "Request body must contain one JSON value.", requestID(r))
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
func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}
func queryMap(r *http.Request) map[string]any {
	m := map[string]any{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 && k != "cursor" && k != "limit" && k != "offset" && !strings.HasPrefix(k, "_") {
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
