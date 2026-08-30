package httpapi

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func FuzzJSONBody(f *testing.F) {
	f.Add([]byte(`{"name":"Bean"}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		r := httptest.NewRequest("POST", "/", bytes.NewReader(data))
		w := httptest.NewRecorder()
		var out map[string]any
		_ = decode(w, r, &out)
	})
}
