// Package uid generates random UUIDv4 identifiers without external state.
package uid

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"sync/atomic"
	"time"
)

var fallback atomic.Uint64

func New() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", time.Now().UnixNano(), fallback.Add(1))))
		copy(b, sum[:16])
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
