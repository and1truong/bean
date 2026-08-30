package fault

import (
	"os"
	"syscall"
)

// Point terminates the process at a named deterministic test-only failure point.
// It is inert unless BEAN_FAULT_POINT exactly matches name.
func Point(name string) {
	if os.Getenv("BEAN_FAULT_POINT") != name {
		return
	}
	if marker := os.Getenv("BEAN_FAULT_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte(name), 0o600)
	}
	_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
}
