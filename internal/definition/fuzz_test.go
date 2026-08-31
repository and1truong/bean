package definition

import (
	"bytes"
	"testing"
)

func FuzzDecode(f *testing.F) {
	f.Add([]byte("apiVersion: bean/v1alpha1\nname: example\n---\nkind: Entity\nname: item\nfields: []\n"))
	f.Add([]byte("{not yaml"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Decode(bytes.NewReader(data))
	})
}
