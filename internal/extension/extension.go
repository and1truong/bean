// Package extension defines Bean's closed typed external-effect contract.
package extension

const (
	TransportHTTP           = "http"
	PermissionNetwork       = "network"
	SideEffectExternalWrite = "external_write"
	AuthenticationNone      = "none"
	AuthenticationBearer    = "bearer"
	IdempotencyRequired     = "required"
	TransactionAfterCommit  = "after_commit"
	FailureRetryThenFail    = "retry_then_fail"

	MinTimeoutSeconds = 1
	MaxTimeoutSeconds = 30
	MinAttempts       = 1
	MaxAttempts       = 10
	MinDelaySeconds   = 1
	MaxDelaySeconds   = 3600
	MaxResponseBytes  = 1 << 20
)

func Transports() []string      { return []string{TransportHTTP} }
func Permissions() []string     { return []string{PermissionNetwork} }
func SideEffects() []string     { return []string{SideEffectExternalWrite} }
func Authentications() []string { return []string{AuthenticationBearer, AuthenticationNone} }
func IdempotencyModes() []string {
	return []string{IdempotencyRequired}
}
func TransactionModes() []string { return []string{TransactionAfterCommit} }
func FailureModes() []string     { return []string{FailureRetryThenFail} }
