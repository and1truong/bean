package httpapi_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/demoseed"
)

func TestActionBatchIsOrderedBoundedAndNonAtomic(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "batch.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	bundle, err := examples.Load("commerce")
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := runtime.Kernel.Active()
	if _, err = demoseed.Run(ctx, runtime.DB, app, 42); err != nil {
		t.Fatal(err)
	}
	rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "order", Columns: []string{"id", "status"}, OrderBy: []dbal.Order{{Column: "id"}}, Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	pending, fulfilled := "", ""
	for _, row := range rows {
		switch row["status"] {
		case "pending_payment":
			pending = row["id"].(string)
		case "fulfilled":
			fulfilled = row["id"].(string)
		}
	}
	if pending == "" || fulfilled == "" {
		t.Fatalf("demo statuses=%v", rows)
	}
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	handler := runtime.HTTP.Handler()
	login := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	var session map[string]any
	decodeResponse(t, login, &session)
	cookie, csrf := login.Result().Cookies()[0], session["csrfToken"].(string)
	batch := map[string]any{"ids": []string{pending, fulfilled}, "values": map[string]any{"status": "paid"}}
	headers := map[string]string{"Idempotency-Key": "advance-selected-orders"}
	response := serveWithHeaders(t, handler, http.MethodPost, "/api/actions/advance_order/batch", batch, cookie, csrf, headers)
	if response.Code != http.StatusOK {
		t.Fatalf("batch status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Data struct {
			Results []struct {
				ID    string `json:"id"`
				OK    bool   `json:"ok"`
				Error any    `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	decodeResponse(t, response, &result)
	if len(result.Data.Results) != 2 || result.Data.Results[0].ID != pending || !result.Data.Results[0].OK || result.Data.Results[1].ID != fulfilled || result.Data.Results[1].OK {
		t.Fatalf("batch result=%+v", result.Data.Results)
	}
	replay := serveWithHeaders(t, handler, http.MethodPost, "/api/actions/advance_order/batch", batch, cookie, csrf, headers)
	var replayResult struct {
		Data struct {
			Results []struct {
				ID string `json:"id"`
				OK bool   `json:"ok"`
			} `json:"results"`
		} `json:"data"`
	}
	decodeResponse(t, replay, &replayResult)
	if replay.Code != http.StatusOK || len(replayResult.Data.Results) != 2 || replayResult.Data.Results[0].ID != pending || !replayResult.Data.Results[0].OK {
		t.Fatalf("batch replay status=%d result=%+v", replay.Code, replayResult.Data.Results)
	}
	updated, err := runtime.DB.Select(ctx, dbal.Select{Table: "order", Columns: []string{"id", "status"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: pending}, Limit: 1})
	if err != nil || len(updated) != 1 || updated[0]["status"] != "paid" {
		t.Fatalf("successful item rolled back: rows=%v err=%v", updated, err)
	}
	tooMany := make([]string, 201)
	for index := range tooMany {
		tooMany[index] = pending + string(rune(index+1))
	}
	response = serve(t, handler, http.MethodPost, "/api/actions/advance_order/batch", map[string]any{"ids": tooMany, "values": map[string]any{"status": "paid"}}, cookie, csrf)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized batch status=%d body=%s", response.Code, response.Body.String())
	}
}
