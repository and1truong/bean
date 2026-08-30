package render_test

import (
	"bytes"
	"testing"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/render"
)

func TestRedactionIsEquivalentAcrossSerializers(t *testing.T) {
	row := dbal.Row{"id": "1", "title": "Visible", "secret": "never-leak"}
	policy.Redact(row, []string{"secret"})
	serializers := []func([]dbal.Row) ([]byte, error){render.JSON, render.CSV, func(rows []dbal.Row) ([]byte, error) { return render.RSS("feed", "https://example.test", rows) }}
	for _, serialize := range serializers {
		body, e := serialize([]dbal.Row{row})
		if e != nil || bytes.Contains(body, []byte("never-leak")) || bytes.Contains(body, []byte("secret")) {
			t.Fatalf("body=%s err=%v", body, e)
		}
	}
}
