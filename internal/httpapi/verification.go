package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/authmail"
)

func (s *Server) verificationRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	app, ok := s.Kernel.Active()
	if !ok || !app.EmailVerificationEnabled() || s.Actions.AuthMail == nil {
		problem(w, 404, "not_found", "Email verification is not available.", requestID(r))
		return
	}
	if !s.recoveryLimiter.allow(s.clientIP(r)) {
		problem(w, 429, "rate_limited", "Too many email requests. Try again later.", requestID(r))
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
		if err := s.Actions.RequestEmailVerification(r.Context(), app, email); err != nil {
			problem(w, 503, "unavailable", "Email requests are temporarily unavailable.", requestID(r))
			return
		}
	}
	write(w, 202, map[string]string{"message": "If this account needs verification, an email will be sent."})
}
func (s *Server) verificationConfirm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	app, ok := s.Kernel.Active()
	if !ok || !app.EmailVerificationEnabled() || s.Actions.AuthMail == nil {
		problem(w, 404, "not_found", "Email verification is not available.", requestID(r))
		return
	}
	if !s.recoveryResetLimiter.allow(s.clientIP(r)) {
		problem(w, 429, "rate_limited", "Too many token attempts. Try again later.", requestID(r))
		return
	}
	var input action.EmailConfirmation
	if !decode(w, r, &input) {
		return
	}
	if err := s.Actions.ConfirmEmail(r.Context(), app, input); err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
