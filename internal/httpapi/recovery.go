package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/authmail"
)

const recoveryMessage = "If this address belongs to an account, a reset link will be sent."

func (s *Server) recoveryRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	app, ok := s.Kernel.Active()
	if !ok || !app.PasswordRecoveryEnabled() || s.Actions.AuthMail == nil {
		problem(w, 404, "not_found", "Password recovery is not available.", requestID(r))
		return
	}
	if !s.recoveryLimiter.allow(s.clientIP(r)) {
		problem(w, 429, "rate_limited", "Too many recovery requests. Try again later.", requestID(r))
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &input) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !authmail.ValidEmail(email) {
		problem(w, 400, "invalid_email", "Enter a valid email address.", requestID(r))
		return
	}
	digest := sha256.Sum256([]byte(email))
	if s.recoveryDestinationLimiter.allow(hex.EncodeToString(digest[:])) {
		if err := s.Actions.RequestPasswordRecovery(r.Context(), app, email); err != nil {
			problem(w, 503, "unavailable", "Recovery requests are temporarily unavailable.", requestID(r))
			return
		}
	}
	write(w, 202, map[string]string{"message": recoveryMessage})
}
func (s *Server) recoveryReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	app, ok := s.Kernel.Active()
	if !ok || !app.PasswordRecoveryEnabled() || s.Actions.AuthMail == nil {
		problem(w, 404, "not_found", "Password recovery is not available.", requestID(r))
		return
	}
	if !s.recoveryResetLimiter.allow(s.clientIP(r)) {
		problem(w, 429, "rate_limited", "Too many reset attempts. Try again later.", requestID(r))
		return
	}
	var input action.RecoveryReset
	if !decode(w, r, &input) {
		return
	}
	if err := s.Actions.ResetPasswordWithToken(r.Context(), app, input); err != nil {
		respondError(w, r, err)
		return
	}
	// The token grants authority for its own account, not for any incidental
	// browser cookie. Do not mutate another account's cookie or auto-login.
	write(w, 200, map[string]bool{"ok": true})
}
