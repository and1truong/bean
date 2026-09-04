package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/audit"
	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/block"
	"github.com/beanruntime/bean/internal/compiler"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/kernel"
	beanmenu "github.com/beanruntime/bean/internal/menu"
	"github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/render"
	"github.com/beanruntime/bean/internal/sequence"
	"github.com/beanruntime/bean/internal/uiassets"
	"github.com/beanruntime/bean/internal/uid"
	"github.com/beanruntime/bean/internal/view"
	"github.com/beanruntime/bean/internal/webform"
)

type Server struct {
	Kernel         *kernel.Kernel
	Store          *release.Store
	Auth           auth.Service
	Actions        action.Service
	Views          view.Service
	SecureCookies  bool
	TrustedProxies []netip.Prefix
	Logger         *slog.Logger
	limiter        *loginLimiter
	signupLimiter  *loginLimiter
}

func (s *Server) Handler() http.Handler {
	s.limiter = newLoginLimiter()
	s.signupLimiter = newLoginLimiter()
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
	mux.HandleFunc("GET /api/menus/{name}", s.menu)
	mux.HandleFunc("GET /api/files/{id}", s.file)
	mux.HandleFunc("POST /api/actions/{name}", s.action)
	mux.HandleFunc("POST /api/actions/{name}/batch", s.actionBatch)
	mux.HandleFunc("POST /api/webforms/{name}/submit", s.form)
	mux.HandleFunc("GET /api/admin/definitions", s.definitions)
	mux.HandleFunc("GET /api/admin/manifest", s.adminManifest)
	mux.HandleFunc("GET /api/admin/resources/{name}", s.adminResourceList)
	mux.HandleFunc("GET /api/admin/resources/{name}/{id}", s.adminResourceRecord)
	mux.HandleFunc("GET /api/admin/navigation/{entity}/{id}", s.adminNavigation)
	mux.HandleFunc("POST /api/admin/definitions", s.saveDefinition)
	mux.HandleFunc("PUT /api/admin/definitions/{id}", s.saveDefinition)
	mux.HandleFunc("POST /api/admin/explore/preview", s.explorePreview)
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
	authNavigation := a.LocalRegistration != nil || len(a.Roles) > 0
	for _, definition := range a.Policies {
		authNavigation = authNavigation || definition.Authenticated || definition.Owner || definition.Tenant || len(definition.ReadRoles) > 0 || len(definition.WriteRoles) > 0 || authenticationCondition(definition.Condition)
	}
	for _, entity := range a.Entities {
		authNavigation = authNavigation || entity.Owner || entity.Tenant
	}
	write(w, 200, map[string]any{"appId": a.AppID, "releaseId": a.ReleaseID, "version": a.Version, "authNavigation": authNavigation, "theme": a.Theme, "entities": a.Entities, "views": a.Views, "actions": a.Actions, "lifecycles": a.Lifecycles, "filters": a.Filters, "webforms": a.Webforms, "pages": a.Pages, "localRegistration": a.LocalRegistration})
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
	write(w, 200, map[string]any{"appId": a.AppID, "releaseId": a.ReleaseID, "version": a.Version, "entities": a.Entities, "views": a.Views, "actions": a.Actions, "lifecycles": a.Lifecycles, "adminResources": a.AdminResources, "systemAdmin": role(s.ctx(r).User.Roles, "administrator")})
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
	write(w, 200, map[string]any{"data": result.Rows, "nextCursor": result.NextCursor, "shape": result.Shape})
}
func (s *Server) adminNavigation(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	entityName, targetID := r.PathValue("entity"), r.PathValue("id")
	entity, exists := a.Entities[entityName]
	if !exists || entity.Navigation == nil {
		problem(w, 404, "not_found", "Entity navigation not found.", requestID(r))
		return
	}
	existing := []dbal.Row{}
	if targetID != "_new" {
		where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "target_entity", Value: entityName}, dbal.Predicate{Op: dbal.OpEQ, Column: "target_id", Value: targetID})
		var err error
		existing, err = s.Actions.DB.Select(r.Context(), dbal.Select{Table: beanmenu.PlacementTable, Where: &where, OrderBy: []dbal.Order{{Column: "menu_name"}, {Column: "owner_id"}}, Limit: beanmenu.MaxEditorInstances + 1})
		if err != nil {
			respondError(w, r, err)
			return
		}
		if len(existing) > beanmenu.MaxEditorInstances {
			problem(w, 409, "navigation_limit", "Record has too many Menu placements to edit safely.", requestID(r))
			return
		}
	}
	instances := []map[string]any{}
	truncated := false
	for menuIndex, menuName := range entity.Navigation.Menus {
		definition := a.Menus[menuName]
		if definition.Owner == nil {
			continue
		}
		var ownerResource appir.AdminResource
		for _, candidate := range a.AdminResources {
			if candidate.Entity == definition.Owner.Entity && (ownerResource.Name == "" || candidate.Name < ownerResource.Name) {
				ownerResource = candidate
			}
		}
		if ownerResource.Name == "" {
			continue
		}
		remaining := beanmenu.MaxEditorInstances - len(instances)
		if remaining <= 0 {
			truncated = true
			break
		}
		placementByOwner := map[string]dbal.Row{}
		ownerRows := []dbal.Row{}
		seenOwners := map[string]bool{}
		for _, placement := range existing {
			if fmt.Sprint(placement["menu_name"]) != menuName {
				continue
			}
			ownerID := fmt.Sprint(placement["owner_id"])
			result, err := s.Views.RunPage(r.Context(), a, ownerResource.View, view.Params{RecordID: ownerID, Limit: 1}, s.ctx(r))
			if err != nil || len(result.Rows) == 0 {
				continue
			}
			ownerRows = append(ownerRows, result.Rows[0])
			seenOwners[ownerID] = true
			placementByOwner[ownerID] = placement
		}
		laterMenus := stringSet(entity.Navigation.Menus[menuIndex+1:])
		reserved := 0
		for _, placement := range existing {
			if laterMenus[fmt.Sprint(placement["menu_name"])] {
				reserved++
			}
		}
		ownerCapacity := remaining - reserved
		ownerLimit := ownerCapacity + 1
		if maximum := a.Views[ownerResource.View].MaxLimit; maximum > 0 && ownerLimit > maximum {
			ownerLimit = maximum
		}
		owners, err := s.Views.RunPage(r.Context(), a, ownerResource.View, view.Params{Limit: ownerLimit}, s.ctx(r))
		if err != nil {
			respondError(w, r, err)
			return
		}
		for _, owner := range owners.Rows {
			if len(ownerRows) >= ownerCapacity {
				truncated = true
				break
			}
			ownerID := fmt.Sprint(owner["id"])
			if !seenOwners[ownerID] {
				ownerRows = append(ownerRows, owner)
				seenOwners[ownerID] = true
			}
		}
		truncated = truncated || owners.NextCursor != "" || len(owners.Rows) > ownerCapacity
		for _, owner := range ownerRows {
			ownerID := fmt.Sprint(owner["id"])
			tree, treeErr := beanmenu.DynamicTree(r.Context(), s.Actions.DB, a, menuName, ownerID, s.ctx(r))
			if treeErr != nil {
				continue
			}
			instance := map[string]any{"menu": menuName, "ownerId": ownerID, "ownerLabel": fmt.Sprint(owner[ownerResource.LabelField]), "items": tree}
			if placement, placed := placementByOwner[ownerID]; placed {
				instance["placement"] = map[string]any{"id": placement["id"], "parentId": placement["parent_id"], "weight": placement["weight"], "labelOverride": placement["label_override"]}
			}
			instances = append(instances, instance)
		}
	}
	write(w, 200, map[string]any{"instances": instances, "truncated": truncated})
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
func authenticationCondition(condition *expr.Expr) bool {
	if condition == nil {
		return false
	}
	for _, value := range []*expr.Value{condition.Left, condition.Right} {
		if value != nil && (value.Source == "user" || value.Source == "tenant") {
			return true
		}
	}
	for i := range condition.Args {
		if authenticationCondition(&condition.Args[i]) {
			return true
		}
	}
	return false
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
	ip := s.clientIP(r)
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
	protected := false
	if a, active := s.Kernel.Active(); active {
		if definition, _, matched := page.Match(a, r.URL.Query().Get("path")); matched {
			protected = page.Protected(a, definition)
		} else if definition, matched := sequence.Match(a, r.URL.Query().Get("path")); matched {
			protected = sequence.Protected(a, definition)
		} else if matched, found := view.MatchPageDisplay(a, r.URL.Query().Get("path")); found {
			viewDefinition := a.Views[matched.View]
			entity := a.Entities[viewDefinition.Entity]
			policyDefinition := a.Policies[policy.EffectiveViewPolicyName(viewDefinition, entity)]
			protected = entity.Owner || entity.Tenant || policyDefinition.Authenticated || policyDefinition.Owner || policyDefinition.Tenant || len(policyDefinition.ReadRoles) > 0 || authenticationCondition(policyDefinition.Condition)
		}
	}
	_, session, ok := s.requestContext(r)
	if ok {
		if !csrf(r, session.CSRF) {
			problem(w, 403, "csrf", "CSRF validation failed.", requestID(r))
			return
		}
		_ = s.Auth.Logout(r.Context(), session.ID)
	}
	http.SetCookie(w, &http.Cookie{Name: "bean_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.SecureCookies})
	write(w, 200, map[string]bool{"ok": true, "protected": protected})
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
	bound, display, viewContext, e := s.boundViewInputs(r, a, r.PathValue("name"))
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
	search := r.URL.Query().Get("q")
	delete(filters, "q")
	searchFields := []string{}
	if search != "" {
		searchFields = a.Views[r.PathValue("name")].Search.Fields
		if len(searchFields) == 0 && display != nil {
			searchFields = display.Renderer.SearchFields
		}
		if len(searchFields) == 0 {
			problem(w, 400, "invalid_query", "Search is not configured for this View block.", requestID(r))
			return
		}
	}
	if display != nil {
		enforceControls := r.URL.Query().Get("_display") != ""
		if blockDefinition, exists := a.Blocks[r.URL.Query().Get("_block")]; exists && !strings.HasPrefix(blockDefinition.Display, "_block_") {
			enforceControls = true
		}
		allowed := map[string]bool{}
		for _, control := range display.Controls {
			allowed[control.Filter] = true
			if _, supplied := filters[control.Filter]; !supplied && control.Default != nil {
				filters[control.Filter] = control.Default
			}
		}
		for name := range bound {
			allowed[name] = true
		}
		if blockName := r.URL.Query().Get("_block"); blockName != "" {
			if pageDefinition, _, matched := page.Match(a, r.URL.Query().Get("_page")); matched {
				for _, pageFilter := range pageDefinition.Filters {
					for _, target := range pageFilter.Targets {
						if target.Block == blockName {
							allowed[target.Filter] = true
						}
					}
				}
			}
		}
		for name := range filters {
			if enforceControls && !allowed[name] {
				problem(w, 400, "invalid_query", "Filter is not configured for this View display.", requestID(r))
				return
			}
		}
		if display.Renderer.Type == "detail" {
			limit = a.Views[r.PathValue("name")].MaxLimit
		} else {
			limit = display.Pager.PageSize
		}
	}
	if blockName := r.URL.Query().Get("_block"); blockName != "" {
		blockDefinition := a.Blocks[blockName]
		specification, registered := block.Lookup(blockDefinition.Type)
		if registered && specification.InputTarget == block.ResourceInputTarget {
			allowed := stringSet(blockDefinition.Filters)
			for name := range blockDefinition.Bindings {
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
	result, e := s.Views.RunPage(r.Context(), a, r.PathValue("name"), view.Params{Filter: filters, Search: search, SearchFields: searchFields, Limit: limit, Offset: offset, Cursor: r.URL.Query().Get("cursor")}, viewContext)
	if e != nil {
		respondError(w, r, e)
		return
	}
	if result.NextCursor != "" {
		w.Header().Set("Bean-Next-Cursor", result.NextCursor)
	}
	write(w, 200, map[string]any{"data": result.Rows, "nextCursor": result.NextCursor, "shape": result.Shape})
}
func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	a, active := s.Kernel.Active()
	if !active || !s.fileAllowed(r, a, r.PathValue("id")) {
		problem(w, 404, "not_found", "File not found.", requestID(r))
		return
	}
	rows, err := s.Actions.DB.Select(r.Context(), dbal.Select{Table: "bean_blob", Columns: []string{"file_name", "content_type", "content", "created_at"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: r.PathValue("id")}, Limit: 1})
	if err != nil {
		respondError(w, r, err)
		return
	}
	if len(rows) == 0 {
		problem(w, 404, "not_found", "File not found.", requestID(r))
		return
	}
	content, err := base64.StdEncoding.DecodeString(fmt.Sprint(rows[0]["content"]))
	if err != nil {
		problem(w, 500, "internal", "File content is invalid.", requestID(r))
		return
	}
	contentType := fmt.Sprint(rows[0]["content_type"])
	if _, _, err = mime.ParseMediaType(contentType); err != nil {
		contentType = "application/octet-stream"
	}
	name := safeFilename(fmt.Sprint(rows[0]["file_name"]))
	created, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(rows[0]["created_at"]))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, created, bytes.NewReader(content))
}

func (s *Server) fileAllowed(r *http.Request, a *appir.App, id string) bool {
	viewName := r.URL.Query().Get("view")
	viewDefinition, exists := a.Views[viewName]
	if !exists {
		return false
	}
	bound, _, requestContext, err := s.boundViewInputs(r, a, viewName)
	if err != nil {
		return false
	}
	filters := queryMap(r)
	delete(filters, "view")
	for name, value := range bound {
		if _, collision := filters[name]; collision {
			return false
		}
		filters[name] = value
	}
	for _, entity := range a.Entities {
		if viewDefinition.Entity != entity.Name {
			continue
		}
		for _, definition := range entity.Fields {
			if definition.Type != "file" || !stringSet(viewDefinition.Fields)[definition.Name] {
				continue
			}
			rows, err := s.Actions.DB.Select(r.Context(), dbal.Select{Table: entity.Name, Columns: []string{"id"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: definition.Name, Value: id}, Limit: 1})
			if err != nil || len(rows) == 0 {
				continue
			}
			result, runErr := s.Views.RunPage(r.Context(), a, viewName, view.Params{Filter: filters, RecordID: fmt.Sprint(rows[0]["id"]), Limit: 1}, requestContext)
			return runErr == nil && len(result.Rows) == 1 && fmt.Sprint(result.Rows[0][definition.Name]) == id
		}
	}
	return false
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
	definition := a.Actions[r.PathValue("name")]
	in, decoded := decodeWebform(w, r, definition)
	if !decoded {
		return
	}
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		in["_idempotencyKey"] = key
	}
	if definition.Operation == "register_local_user" && (a.LocalRegistration == nil || a.LocalRegistration.Action != definition.Name) {
		problem(w, 404, "not_found", "Action not found.", requestID(r))
		return
	} else if definition.Operation == "register_local_user" && !s.signupLimiter.allow(s.clientIP(r)) {
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
func (s *Server) actionBatch(w http.ResponseWriter, r *http.Request) {
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
	name := r.PathValue("name")
	definition, exists := a.Actions[name]
	if !exists || definition.Operation == "register_local_user" {
		problem(w, 404, "not_found", "Action not found.", requestID(r))
		return
	}
	var input struct {
		IDs    []string       `json:"ids"`
		Values map[string]any `json:"values"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.IDs) == 0 || len(input.IDs) > 200 {
		problem(w, 400, "invalid_batch", "Action batch must contain between 1 and 200 record IDs.", requestID(r))
		return
	}
	seen := map[string]bool{}
	for _, id := range input.IDs {
		if id == "" || seen[id] {
			problem(w, 400, "invalid_batch", "Action batch record IDs must be nonempty and unique.", requestID(r))
			return
		}
		seen[id] = true
	}
	results := make([]map[string]any, 0, len(input.IDs))
	batchKey := r.Header.Get("Idempotency-Key")
	for _, id := range input.IDs {
		values := map[string]any{"id": id}
		for key, value := range input.Values {
			if key != "id" {
				values[key] = value
			}
		}
		if batchKey != "" {
			values["_idempotencyKey"] = batchIdempotencyKey(batchKey, id)
		}
		result, err := s.Actions.Execute(r.Context(), a, name, values, c)
		if err == nil {
			results = append(results, map[string]any{"id": id, "ok": true, "data": result})
			continue
		}
		code, message := "internal", "Action failed."
		if typed, typedOK := err.(*dbal.Error); typedOK {
			code, message = string(typed.Code), typed.Message
		}
		results = append(results, map[string]any{"id": id, "ok": false, "error": map[string]string{"code": code, "message": message}})
	}
	write(w, 200, map[string]any{"data": map[string]any{"results": results}})
}

func batchIdempotencyKey(batchKey, recordID string) string {
	sum := sha256.Sum256([]byte(batchKey + "\x00" + recordID))
	return "batch:" + hex.EncodeToString(sum[:])
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
		if !s.signupLimiter.allow(s.clientIP(r)) {
			problem(w, 429, "rate_limited", "Too many signup attempts.", requestID(r))
			return
		}
	}
	c, session, authed := s.requestContext(r)
	if authed && !csrf(r, session.CSRF) {
		problem(w, 403, "csrf", "CSRF validation failed.", requestID(r))
		return
	}
	actionDefinition = a.Actions[f.Action]
	in, decoded := decodeWebform(w, r, actionDefinition)
	if !decoded {
		return
	}
	bound, formContext, e := s.boundBlockInputs(r, a, "webform", f.Name)
	if e != nil {
		problem(w, 400, "bound_input", e.Error(), requestID(r))
		return
	}
	c = formContext
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
	token, e := s.Store.DraftToken(r.Context(), "default")
	if e != nil {
		respondError(w, r, e)
		return
	}
	defs, e := s.Store.Draft(r.Context(), "default")
	if e != nil {
		respondError(w, r, e)
		return
	}
	w.Header().Set("ETag", `"`+token+`"`)
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
	var e error
	if expected := r.Header.Get("If-Match"); expected != "" {
		if len(expected) < 2 || expected[0] != '"' || expected[len(expected)-1] != '"' {
			problem(w, 400, "invalid_precondition", "If-Match must contain the quoted draft token.", requestID(r))
			return
		}
		e = s.Store.SaveDefinitionIfDraftToken(r.Context(), "default", d, expected[1:len(expected)-1])
	} else {
		e = s.Store.SaveDefinition(r.Context(), "default", d)
	}
	if e != nil {
		respondError(w, r, e)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}

type explorePreviewRequest struct {
	Name   string         `json:"name"`
	Spec   map[string]any `json:"spec"`
	Filter map[string]any `json:"filter"`
	Search string         `json:"search"`
	Limit  int            `json:"limit"`
	Cursor string         `json:"cursor"`
}

func (s *Server) explorePreview(w http.ResponseWriter, r *http.Request) {
	if !s.adminMutation(w, r) {
		return
	}
	var input explorePreviewRequest
	if !decode(w, r, &input) {
		return
	}
	active, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	compiled := compiler.CompileViewCandidate(active, input.Name, input.Spec)
	if len(compiled.Diagnostics) > 0 {
		write(w, 400, map[string]any{"valid": false, "diagnostics": compiled.Diagnostics})
		return
	}
	candidate := compiled.App.Views[input.Name]
	result, err := s.Views.RunPage(r.Context(), compiled.App, input.Name, view.Params{
		Filter: input.Filter, Search: input.Search, SearchFields: candidate.Search.Fields,
		Limit: input.Limit, Cursor: input.Cursor,
	}, s.ctx(r))
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, map[string]any{"valid": true, "view": candidate, "data": result.Rows, "nextCursor": result.NextCursor, "shape": result.Shape})
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
	var active *appir.App
	if current, ok := s.Kernel.Active(); ok {
		active = current
	}
	write(w, 200, map[string]any{"valid": len(result.Diagnostics) == 0, "diagnostics": result.Diagnostics, "schema": result.Schema, "migration": migrationPlan, "changes": compiler.DefinitionDiff(active, result.App)})
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
	requestContext := s.ctx(r)
	entityName, entityID := r.URL.Query().Get("entity"), r.URL.Query().Get("id")
	if !role(requestContext.User.Roles, "administrator") {
		a, active := s.Kernel.Active()
		if !active || entityName == "" || entityID == "" {
			problem(w, 403, "forbidden", "Administrator access is required for unscoped audit history.", requestID(r))
			return
		}
		authorized := false
		for _, resource := range a.AdminResources {
			if resource.Entity != entityName {
				continue
			}
			result, err := s.Views.RunPage(r.Context(), a, resource.View, view.Params{RecordID: entityID, Limit: 1}, requestContext)
			if err == nil && len(result.Rows) > 0 {
				authorized = true
				break
			}
		}
		if !authorized {
			problem(w, 403, "forbidden", "Audit history is outside the permitted application scope.", requestID(r))
			return
		}
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

func stringListContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	requestContext := s.ctx(r)
	requestContext.Route = r.URL.Query().Get("path")
	p, params, ok := page.Match(a, requestContext.Route)
	if !ok {
		if matched, found := sequence.Match(a, r.URL.Query().Get("path")); found {
			tree, allowed, err := sequence.Node(a, matched, requestContext)
			if err != nil {
				problem(w, 400, "render_context", "Required sequence render context is missing.", requestID(r))
				return
			}
			if !allowed {
				problem(w, 404, "not_found", "Sequence not found.", requestID(r))
				return
			}
			write(w, 200, map[string]any{"tree": tree})
			return
		}
		matched, found := view.MatchPageDisplay(a, r.URL.Query().Get("path"))
		if !found {
			problem(w, 404, "not_found", "Page not found.", requestID(r))
			return
		}
		query := map[string]string{}
		for key := range r.URL.Query() {
			query[key] = r.URL.Query().Get(key)
		}
		if _, err := view.ResolveDisplayBindings(matched.Display, matched.Params, query, requestContext); err != nil {
			problem(w, 400, "missing_context", "Required View display context is missing.", requestID(r))
			return
		}
		tree := view.DisplayPageNode(a, matched)
		menuName, ownerID := r.URL.Query().Get("_menu"), r.URL.Query().Get("_owner")
		if menuName != "" || ownerID != "" {
			entity := a.Entities[a.Views[matched.View].Entity]
			definition, menuExists := a.Menus[menuName]
			if menuName == "" || ownerID == "" || !menuExists || definition.Owner == nil || entity.Navigation == nil || !stringListContains(entity.Navigation.Menus, menuName) {
				problem(w, 400, "invalid_menu_context", "View display Menu context is invalid.", requestID(r))
				return
			}
			menuNode := render.Node{Component: "MenuBlock", Props: map[string]any{"name": "@display-menu/" + menuName, "menu": menuName, "profile": definition.Profile, "ownerEntity": definition.Owner.Entity, "ownerID": ownerID}}
			tree.Children = append([]render.Node{menuNode}, tree.Children...)
		}
		write(w, 200, map[string]any{"tree": tree})
		return
	}
	if p.Policy != "" {
		definition, exists := a.Policies[p.Policy]
		if !exists || !policy.Can(definition, false, requestContext, nil) {
			problem(w, 404, "not_found", "Page not found.", requestID(r))
			return
		}
	}
	query := map[string]string{}
	for key := range r.URL.Query() {
		query[key] = r.URL.Query().Get(key)
	}
	ctx, e := page.ResolveContext(p, params, query, requestContext)
	if e != nil {
		problem(w, 400, "missing_context", "Required page context is missing.", requestID(r))
		return
	}
	requestContext.RouteParams = params
	requestContext.Values = ctx
	tree, allowed, e := page.Node(a, p, ctx, requestContext)
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

func (s *Server) menu(w http.ResponseWriter, r *http.Request) {
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	definition, exists := a.Menus[r.PathValue("name")]
	if !exists || definition.Owner == nil {
		problem(w, 404, "not_found", "Menu not found.", requestID(r))
		return
	}
	requestContext := s.ctx(r)
	requestContext.Route = r.URL.Query().Get("_page")
	ownerID := r.URL.Query().Get("_owner")
	if r.URL.Query().Get("_block") != "" {
		inputs, boundContext, err := s.boundBlockInputs(r, a, "menu", definition.Name)
		if err != nil {
			problem(w, 403, "forbidden", "Menu is unavailable for this request.", requestID(r))
			return
		}
		requestContext = boundContext
		ownerID = fmt.Sprint(inputs["owner_id"])
	} else {
		matched, found := view.MatchPageDisplay(a, r.URL.Query().Get("_page"))
		if !found {
			problem(w, 403, "forbidden", "Menu is unavailable for this request.", requestID(r))
			return
		}
		entity := a.Entities[a.Views[matched.View].Entity]
		if entity.Navigation == nil || !stringListContains(entity.Navigation.Menus, definition.Name) {
			problem(w, 403, "forbidden", "Menu is unavailable for this request.", requestID(r))
			return
		}
	}
	tree, err := beanmenu.DynamicTree(r.Context(), s.Actions.DB, a, definition.Name, ownerID, requestContext)
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, map[string]any{"items": tree})
}

func (s *Server) boundViewInputs(r *http.Request, a *appir.App, target string) (map[string]any, *appir.Display, beanctx.Request, error) {
	displayName := r.URL.Query().Get("_display")
	if displayName == "" {
		bound, requestContext, err := s.boundBlockInputs(r, a, "view", target)
		if err != nil {
			return nil, nil, requestContext, err
		}
		blockDefinition, exists := a.Blocks[r.URL.Query().Get("_block")]
		if !exists || blockDefinition.Display == "" {
			return bound, nil, requestContext, nil
		}
		selectedDisplay := blockDefinition.Display
		if requested := r.URL.Query().Get("_viewDisplay"); requested != "" {
			selectedDisplay = requested
		}
		display, exists := a.Views[target].Displays[selectedDisplay]
		if !exists || display.Type != "block" {
			return nil, nil, requestContext, fmt.Errorf("bound View display was not found")
		}
		return bound, &display, requestContext, nil
	}
	if r.URL.Query().Get("_block") != "" || r.URL.Query().Get("_page") == "" {
		return nil, nil, s.ctx(r), fmt.Errorf("page and display context must be supplied together")
	}
	matched, found := view.MatchPageDisplay(a, r.URL.Query().Get("_page"))
	if !found || matched.View != target || matched.Name != displayName {
		return nil, nil, s.ctx(r), fmt.Errorf("bound View display does not match this request")
	}
	query := map[string]string{}
	for key := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {
			query[key] = r.URL.Query().Get(key)
		}
	}
	requestContext := s.ctx(r)
	bound, err := view.ResolveDisplayBindings(matched.Display, matched.Params, query, requestContext)
	if err != nil {
		return nil, nil, requestContext, err
	}
	requestContext.RouteParams = matched.Params
	requestContext.Values = bound
	return bound, &matched.Display, requestContext, nil
}

func (s *Server) boundBlockInputs(r *http.Request, a *appir.App, kind, target string) (map[string]any, beanctx.Request, error) {
	requestContext := s.ctx(r)
	pagePath, blockName := r.URL.Query().Get("_page"), r.URL.Query().Get("_block")
	if pagePath == "" && blockName == "" {
		return nil, requestContext, nil
	}
	if pagePath == "" || blockName == "" {
		return nil, requestContext, fmt.Errorf("page and block context must be supplied together")
	}
	definition, exists := a.Blocks[blockName]
	if !exists || kind == "view" && definition.View != target || kind == "webform" && definition.Webform != target || kind == "menu" && definition.Menu != target {
		return nil, requestContext, fmt.Errorf("bound block does not match this request")
	}
	p, routeParams, matched := page.Match(a, pagePath)
	if !matched {
		item, sequenceMatched := sequence.Match(a, pagePath)
		if !sequenceMatched {
			return nil, requestContext, fmt.Errorf("bound page or sequence was not found")
		}
		if item.Policy != "" && !policy.Can(a.Policies[item.Policy], false, requestContext, nil) {
			return nil, requestContext, fmt.Errorf("bound sequence is unavailable")
		}
		found := false
		for _, frame := range item.Frames {
			panelDefinition := a.Panels[frame.Panel]
			if !panelContainsBlock(panelDefinition, blockName) {
				continue
			}
			found = true
			if panelDefinition.Policy != "" && !policy.Can(a.Policies[panelDefinition.Policy], false, requestContext, nil) {
				continue
			}
			node, allowed, err := block.Node(a, definition, map[string]any{}, requestContext)
			if err == nil && allowed {
				inputs, _ := node.Props["inputs"].(map[string]any)
				return inputs, requestContext, nil
			}
		}
		if !found {
			return nil, requestContext, fmt.Errorf("bound block does not match this request")
		}
		return nil, requestContext, fmt.Errorf("bound block is unavailable")
	}
	if p.Policy != "" && !policy.Can(a.Policies[p.Policy], false, requestContext, nil) {
		return nil, requestContext, fmt.Errorf("bound page is unavailable")
	}
	containingPanels := []appir.Panel{}
	for _, panelName := range p.PanelNames() {
		panelDefinition := a.Panels[panelName]
		if panelContainsBlock(panelDefinition, blockName) {
			containingPanels = append(containingPanels, panelDefinition)
		}
	}
	if len(containingPanels) == 0 {
		return nil, requestContext, fmt.Errorf("bound block does not match this request")
	}
	query := map[string]string{}
	for key := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {
			query[key] = r.URL.Query().Get(key)
		}
	}
	contextValues, err := page.ResolveContext(p, routeParams, query, requestContext)
	if err != nil {
		return nil, requestContext, fmt.Errorf("required bound context is missing")
	}
	requestContext.RouteParams = routeParams
	requestContext.Values = contextValues
	for _, panelDefinition := range containingPanels {
		if panelDefinition.Policy != "" && !policy.Can(a.Policies[panelDefinition.Policy], false, requestContext, nil) {
			continue
		}
		node, allowed, err := block.Node(a, definition, contextValues, requestContext)
		if err == nil && allowed {
			inputs, _ := node.Props["inputs"].(map[string]any)
			return inputs, requestContext, nil
		}
	}
	return nil, requestContext, fmt.Errorf("bound block is unavailable")
}

func panelContainsBlock(panelDefinition appir.Panel, name string) bool {
	for _, region := range panelDefinition.Regions {
		for _, item := range region.OrderedItems() {
			if item.Block == name {
				return true
			}
		}
	}
	return false
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
func decodeWebform(w http.ResponseWriter, r *http.Request, action appir.Action) (map[string]any, bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		var input map[string]any
		return input, decode(w, r, &input)
	}
	fileFields := 0
	for _, definition := range action.Input {
		if definition.Type == "file" {
			fileFields++
		}
	}
	bodyLimit := int64(1 << 20)
	bodyLimit += int64(fileFields) * int64(field.MaxFileBytes)
	r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
	if err := r.ParseMultipartForm(bodyLimit); err != nil {
		problem(w, 400, "invalid_form", "Multipart form is invalid or too large.", requestID(r))
		return nil, false
	}
	defer r.MultipartForm.RemoveAll()
	input := map[string]any{}
	for name, values := range r.MultipartForm.Value {
		definition, declared := action.Input[name]
		if !declared || len(values) != 1 {
			problem(w, 400, "invalid_form", "Multipart form contains an invalid field.", requestID(r))
			return nil, false
		}
		value, err := formValue(values[0], definition)
		if err != nil {
			problem(w, 400, "invalid_form", "Multipart form field has an invalid value.", requestID(r))
			return nil, false
		}
		input[name] = value
	}
	for name, files := range r.MultipartForm.File {
		definition, declared := action.Input[name]
		if !declared || definition.Type != "file" || len(files) != 1 {
			problem(w, 400, "invalid_form", "Multipart form contains an invalid file field.", requestID(r))
			return nil, false
		}
		opened, err := files[0].Open()
		if err != nil {
			problem(w, 400, "invalid_form", "Uploaded file cannot be read.", requestID(r))
			return nil, false
		}
		data, err := io.ReadAll(io.LimitReader(opened, field.MaxFileBytes+1))
		_ = opened.Close()
		if err != nil || len(data) == 0 || len(data) > field.MaxFileBytes {
			problem(w, 400, "invalid_form", "Uploaded file is empty or too large.", requestID(r))
			return nil, false
		}
		contentType := files[0].Header.Get("Content-Type")
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		input[name] = field.Upload{Name: safeFilename(files[0].Filename), ContentType: contentType, Data: data}
	}
	return input, true
}

func formValue(value string, definition appir.Field) (any, error) {
	if definition.Type == "relation" && definition.Relation != nil && (definition.Relation.Kind == "one-to-many" || definition.Relation.Kind == "many-to-many") {
		var parsed []any
		err := json.Unmarshal([]byte(value), &parsed)
		return parsed, err
	}
	switch definition.Type {
	case "integer", "money":
		parsed, err := strconv.ParseInt(value, 10, 64)
		return parsed, err
	case "boolean":
		parsed, err := strconv.ParseBool(value)
		return parsed, err
	case "json":
		var parsed any
		err := json.Unmarshal([]byte(value), &parsed)
		return parsed, err
	default:
		return value, nil
	}
}

func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." {
		return "download"
	}
	return name
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
		case dbal.InvalidQuery, dbal.ResultLimitExceeded:
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
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, peerErr := netip.ParseAddr(host)
	if peerErr == nil && trustedProxy(peer, s.TrustedProxies) {
		chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		for index := len(chain) - 1; index >= 0; index-- {
			address, parseErr := netip.ParseAddr(strings.TrimSpace(chain[index]))
			if parseErr != nil {
				return host
			}
			if !trustedProxy(address, s.TrustedProxies) || index == 0 {
				return address.String()
			}
		}
	}
	return host
}

func trustedProxy(address netip.Addr, configured []netip.Prefix) bool {
	for _, prefix := range configured {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

const (
	loginLimitWindow     = time.Minute
	loginLimitAttempts   = 10
	loginLimitMaxEntries = 10000
	loginLimitOverflow   = "\x00overflow"
)

type loginLimitEntry struct {
	attempts []time.Time
	updated  time.Time
}

type loginLimiter struct {
	mu         sync.Mutex
	attempts   map[string]loginLimitEntry
	lastSweep  time.Time
	maxEntries int
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: map[string]loginLimitEntry{}, maxEntries: loginLimitMaxEntries}
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-loginLimitWindow)
	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) >= loginLimitWindow {
		for key, entry := range l.attempts {
			if entry.updated.Before(cut) {
				delete(l.attempts, key)
			}
		}
		l.lastSweep = now
	}
	key := "ip:" + ip
	if _, exists := l.attempts[key]; !exists && len(l.attempts) >= l.maxEntries {
		key = loginLimitOverflow
		if _, exists = l.attempts[key]; !exists {
			var oldestKey string
			var oldest time.Time
			for candidate, entry := range l.attempts {
				if oldestKey == "" || entry.updated.Before(oldest) {
					oldestKey, oldest = candidate, entry.updated
				}
			}
			delete(l.attempts, oldestKey)
		}
	}
	entry := l.attempts[key]
	active := entry.attempts[:0]
	for _, attempt := range entry.attempts {
		if attempt.After(cut) {
			active = append(active, attempt)
		}
	}
	entry.attempts = active
	entry.updated = now
	if len(active) >= loginLimitAttempts {
		l.attempts[key] = entry
		return false
	}
	entry.attempts = append(entry.attempts, now)
	l.attempts[key] = entry
	return true
}

var _ = fmt.Sprint
