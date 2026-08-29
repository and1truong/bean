package view

import "testing"

func FuzzCursorDecode(f *testing.F) {
	f.Add(encodeCursor(cursor{Version: "bean/appir/v1", View: "items", Signature: "abc", Values: []any{"item-2"}}))
	f.Add("not-a-cursor")
	f.Fuzz(func(t *testing.T, value string) {
		_, _ = decodeCursor(value)
	})
}
