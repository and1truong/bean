package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/bootstrap"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/view"
)

func TestBlogSecurityAndWorkflowContract(t *testing.T) {
	testBlogSecurityAndWorkflowContract(t, filepath.Join(t.TempDir(), "blog.db"))
}

func TestBlogSecurityAndWorkflowPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_BLOG_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_BLOG_POSTGRES_URL to run PostgreSQL blog parity")
	}
	testBlogSecurityAndWorkflowContract(t, databaseURL)
}

func testBlogSecurityAndWorkflowContract(t *testing.T, databaseURL string) {
	t.Helper()
	ctx := context.Background()
	runtime, err := bootstrap.OpenURL(ctx, databaseURL, false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	source, err := examples.Open("blog")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := definition.Decode(source)
	source.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish err=%v diagnostics=%v", publishErr, diagnostics)
	}
	handler := runtime.HTTP.Handler()
	adminLogin := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	var adminSession map[string]any
	decodeResponse(t, adminLogin, &adminSession)
	adminCookie, adminCSRF := adminLogin.Result().Cookies()[0], adminSession["csrfToken"].(string)
	create := func(actionName string, input map[string]any) map[string]any {
		response := serve(t, handler, http.MethodPost, "/api/actions/"+actionName, input, adminCookie, adminCSRF)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", actionName, response.Code, response.Body.String())
		}
		var output struct {
			Data map[string]any `json:"data"`
		}
		decodeResponse(t, response, &output)
		return output.Data
	}
	category := create("category_create", map[string]any{"name": "Engineering", "slug": "engineering"})
	tagOne := create("tag_create", map[string]any{"name": "Go", "slug": "go"})
	tagTwo := create("tag_create", map[string]any{"name": "Runtime", "slug": "runtime"})
	maliciousBody := "## Bean is **ready**.\n\n<script>alert(1)</script>\n\n[bad](javascript:alert(2))"
	post := create("save_post_draft", map[string]any{"title": "Bean ships", "slug": "bean-ships", "excerpt": "A complete slice", "body": maliciousBody, "author_display_name": "Editor", "category_id": category["id"], "tags": []any{tagOne["id"], tagTwo["id"]}})
	for _, path := range []string{"/api/views/published_posts", "/api/views/published_post?slug=bean-ships", "/api/blog/posts", "/rss.xml"} {
		response := serve(t, handler, http.MethodGet, path, nil, nil, "")
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "Bean ships") {
			if response.Code == http.StatusInternalServerError {
				app, _ := runtime.Kernel.Active()
				_, viewErr := runtime.HTTP.Views.Run(ctx, app, "published_post", view.Params{Filter: map[string]any{"slug": "bean-ships"}}, beanctx.Request{})
				t.Fatalf("draft query failed: %v cause=%v", viewErr, errors.Unwrap(viewErr))
			}
			t.Fatalf("draft leaked from %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	published := create("publish_post", map[string]any{"id": post["id"]})
	if published["status"] != "published" || published["published_at"] == nil {
		t.Fatalf("published result=%v", published)
	}
	for _, path := range []string{"/api/views/published_posts", "/api/blog/posts", "/rss.xml", "/api/views/published_posts_by_category?category_slug=engineering", "/api/views/published_posts_by_tag?tag_slug=go"} {
		response := serve(t, handler, http.MethodGet, path, nil, nil, "")
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Bean ships") {
			t.Fatalf("published post missing from %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	formattedPost := serve(t, handler, http.MethodGet, "/api/views/published_post?slug=bean-ships", nil, nil, "")
	var formattedResult struct {
		Data []map[string]any `json:"data"`
	}
	decodeResponse(t, formattedPost, &formattedResult)
	formattedBody := ""
	if len(formattedResult.Data) > 0 {
		formattedBody = fmt.Sprint(formattedResult.Data[0]["body"])
	}
	if formattedPost.Code != http.StatusOK || !strings.Contains(formattedBody, "<strong>ready</strong>") || strings.Contains(strings.ToLower(formattedBody), "<script") || strings.Contains(strings.ToLower(formattedBody), "javascript:") {
		t.Fatalf("unsafe formatted post status=%d body=%s", formattedPost.Code, formattedPost.Body.String())
	}
	storedPosts, err := runtime.DB.Select(ctx, dbal.Select{Table: "post", Columns: []string{"body"}, Limit: 1})
	if err != nil || len(storedPosts) != 1 || !strings.Contains(fmt.Sprint(storedPosts[0]["body"]), "**ready**") {
		t.Fatalf("Markdown source was not preserved: body=%v err=%v", storedPosts, err)
	}

	password := "member-password"
	signupPath := "/api/webforms/signup/submit?_page=/signup&_block=signup_form"
	signup := serveWithHeaders(t, handler, http.MethodPost, signupPath, map[string]any{"display_name": "Ada Member", "email": " ADA@EXAMPLE.TEST ", "password": password, "password_confirmation": password}, nil, "", map[string]string{"Idempotency-Key": "signup-ada"})
	if signup.Code != http.StatusOK || strings.Contains(signup.Body.String(), password) || strings.Contains(signup.Body.String(), "password") {
		t.Fatalf("unsafe signup response status=%d body=%s", signup.Code, signup.Body.String())
	}
	replay := serveWithHeaders(t, handler, http.MethodPost, signupPath, map[string]any{"display_name": "Ada Member", "email": " ADA@EXAMPLE.TEST ", "password": password, "password_confirmation": password}, nil, "", map[string]string{"Idempotency-Key": "signup-ada"})
	if replay.Code != http.StatusOK || replay.Body.String() != signup.Body.String() {
		t.Fatalf("signup replay status=%d body=%s", replay.Code, replay.Body.String())
	}
	escalation := serve(t, handler, http.MethodPost, "/api/actions/register_member", map[string]any{"display_name": "Mallory", "email": "mallory@example.test", "password": password, "password_confirmation": password, "roles": []any{"administrator"}}, nil, "")
	if escalation.Code != http.StatusBadRequest {
		t.Fatalf("role escalation status=%d body=%s", escalation.Code, escalation.Body.String())
	}
	users, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "email", Value: "ada@example.test"}, Limit: 1})
	if err != nil || len(users) != 1 || fmt.Sprint(users[0]["roles"]) != `["member"]` || strings.Contains(fmt.Sprint(users[0]["password_hash"]), password) {
		t.Fatalf("registered user=%v err=%v", users, err)
	}
	memberLogin := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "ada@example.test", "password": password}, nil, "")
	var memberSession map[string]any
	decodeResponse(t, memberLogin, &memberSession)
	memberCookie, memberCSRF := memberLogin.Result().Cookies()[0], memberSession["csrfToken"].(string)
	if memberAdmin := serve(t, handler, http.MethodGet, "/api/admin/manifest", nil, memberCookie, ""); memberAdmin.Code != http.StatusForbidden {
		t.Fatalf("member admin status=%d body=%s", memberAdmin.Code, memberAdmin.Body.String())
	}
	denied := serve(t, handler, http.MethodPost, "/api/actions/publish_post", map[string]any{"id": post["id"]}, memberCookie, memberCSRF)
	if denied.Code != http.StatusConflict {
		t.Fatalf("member publish status=%d body=%s", denied.Code, denied.Body.String())
	}
	commentPath := "/api/webforms/comment_form/submit?_page=/posts/bean-ships&_block=submit_comment_form"
	withoutCSRF := serve(t, handler, http.MethodPost, commentPath, map[string]any{"body": "No token"}, memberCookie, "")
	if withoutCSRF.Code != http.StatusForbidden {
		t.Fatalf("comment without CSRF status=%d body=%s", withoutCSRF.Code, withoutCSRF.Body.String())
	}
	tampered := serve(t, handler, http.MethodPost, commentPath, map[string]any{"body": "Hello", "post_slug": "other-post"}, memberCookie, memberCSRF)
	if tampered.Code != http.StatusBadRequest {
		t.Fatalf("bound tamper status=%d body=%s", tampered.Code, tampered.Body.String())
	}
	submitted := serveWithHeaders(t, handler, http.MethodPost, commentPath, map[string]any{"body": "Thoughtful comment"}, memberCookie, memberCSRF, map[string]string{"Idempotency-Key": "comment-1"})
	if submitted.Code != http.StatusOK {
		t.Fatalf("comment status=%d body=%s", submitted.Code, submitted.Body.String())
	}
	comments, err := runtime.DB.Select(ctx, dbal.Select{Table: "comment", Limit: 10})
	if err != nil || len(comments) != 1 || comments[0]["status"] != "pending" || comments[0]["owner_id"] == nil || comments[0]["author_display_name"] != "Ada Member" {
		t.Fatalf("pending comments=%v err=%v", comments, err)
	}
	queuePage := "/blog/" + fmt.Sprint(post["id"]) + "/comments"
	if response := serve(t, handler, http.MethodGet, "/api/system/page?path="+queuePage, nil, memberCookie, ""); response.Code != http.StatusNotFound {
		t.Fatalf("member scoped queue status=%d body=%s", response.Code, response.Body.String())
	}
	queueView := "/api/views/comment_admin?_page=" + queuePage + "&_block=post_comment_queue"
	if response := serve(t, handler, http.MethodGet, queueView+"&status=pending", nil, adminCookie, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Thoughtful comment") {
		t.Fatalf("scoped pending queue status=%d body=%s", response.Code, response.Body.String())
	}
	if response := serve(t, handler, http.MethodGet, queueView+"&owner_id=unconfigured", nil, adminCookie, ""); response.Code != http.StatusBadRequest {
		t.Fatalf("unconfigured scoped filter status=%d body=%s", response.Code, response.Body.String())
	}
	if response := serve(t, handler, http.MethodGet, queueView+"&post_id="+fmt.Sprint(post["id"]), nil, adminCookie, ""); response.Code != http.StatusBadRequest {
		t.Fatalf("scoped parent tamper status=%d body=%s", response.Code, response.Body.String())
	}
	memberDirect := serve(t, handler, http.MethodGet, "/api/views/comment_list", nil, memberCookie, "")
	if memberDirect.Code != http.StatusNotFound || strings.Contains(memberDirect.Body.String(), "Thoughtful comment") {
		t.Fatalf("pending comment leaked from direct View: status=%d body=%s", memberDirect.Code, memberDirect.Body.String())
	}
	publicCommentsPath := "/api/views/approved_comments?_page=/posts/bean-ships&_block=post_comments"
	if response := serve(t, handler, http.MethodGet, publicCommentsPath, nil, nil, ""); response.Code != http.StatusOK || strings.Contains(response.Body.String(), "Thoughtful comment") {
		t.Fatalf("pending comment leaked: status=%d body=%s", response.Code, response.Body.String())
	}
	boundViewTamper := serve(t, handler, http.MethodGet, publicCommentsPath+"&post_slug=other-post", nil, nil, "")
	if boundViewTamper.Code != http.StatusBadRequest {
		t.Fatalf("bound View tamper status=%d body=%s", boundViewTamper.Code, boundViewTamper.Body.String())
	}
	memberModeration := serve(t, handler, http.MethodPost, "/api/actions/approve_comment", map[string]any{"id": comments[0]["id"]}, memberCookie, memberCSRF)
	if memberModeration.Code != http.StatusConflict {
		t.Fatalf("member moderation status=%d body=%s", memberModeration.Code, memberModeration.Body.String())
	}
	create("approve_comment", map[string]any{"id": comments[0]["id"]})
	if response := serve(t, handler, http.MethodGet, queueView+"&status=pending", nil, adminCookie, ""); response.Code != http.StatusOK || strings.Contains(response.Body.String(), "Thoughtful comment") {
		t.Fatalf("approved comment remained pending status=%d body=%s", response.Code, response.Body.String())
	}
	if response := serve(t, handler, http.MethodGet, queueView+"&status=approved", nil, adminCookie, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Thoughtful comment") {
		t.Fatalf("approved scoped filter status=%d body=%s", response.Code, response.Body.String())
	}
	if response := serve(t, handler, http.MethodGet, publicCommentsPath, nil, nil, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Thoughtful comment") {
		t.Fatalf("approved comment missing: status=%d body=%s", response.Code, response.Body.String())
	}
	rejectedSubmission := serve(t, handler, http.MethodPost, commentPath, map[string]any{"body": "Rejected comment"}, memberCookie, memberCSRF)
	if rejectedSubmission.Code != http.StatusOK {
		t.Fatalf("rejected comment setup status=%d body=%s", rejectedSubmission.Code, rejectedSubmission.Body.String())
	}
	comments, err = runtime.DB.Select(ctx, dbal.Select{Table: "comment", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "body", Value: "Rejected comment"}, Limit: 1})
	if err != nil || len(comments) != 1 {
		t.Fatalf("rejected comment setup=%v err=%v", comments, err)
	}
	create("reject_comment", map[string]any{"id": comments[0]["id"]})
	if response := serve(t, handler, http.MethodGet, publicCommentsPath, nil, nil, ""); response.Code != http.StatusOK || strings.Contains(response.Body.String(), "Rejected comment") {
		t.Fatalf("rejected comment leaked: status=%d body=%s", response.Code, response.Body.String())
	}
	for _, table := range []string{"bean_audit", "bean_idempotency"} {
		rows, selectErr := runtime.DB.Select(ctx, dbal.Select{Table: table, Limit: 100})
		encoded, _ := json.Marshal(rows)
		if selectErr != nil || bytes.Contains(encoded, []byte(password)) {
			t.Fatalf("password leaked in %s: %s err=%v", table, encoded, selectErr)
		}
	}
	for attempt := 0; attempt < 8; attempt++ {
		response := serve(t, handler, http.MethodPost, "/api/actions/register_member", map[string]any{"display_name": "Blocked", "email": "blocked@example.test", "password": password, "password_confirmation": password, "roles": []any{"administrator"}}, nil, "")
		if attempt < 7 && response.Code == http.StatusTooManyRequests {
			t.Fatalf("signup throttled early on attempt %d", attempt)
		}
		if attempt == 7 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("signup throttle status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

func serveWithHeaders(t *testing.T, handler http.Handler, method, path string, body any, cookie *http.Cookie, csrf string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	encoded, _ := json.Marshal(body)
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
