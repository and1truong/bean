package httpapi

import (
	"net/http"

	"github.com/beanruntime/bean/internal/action"
)

func (s *Server) accountPassword(w http.ResponseWriter, r *http.Request) {
	s.accountMutation(w, r, true)
}
func (s *Server) accountSessions(w http.ResponseWriter, r *http.Request) {
	s.accountMutation(w, r, false)
}
func (s *Server) accountMutation(w http.ResponseWriter, r *http.Request, password bool) {
	_, session, ok := s.requestContext(r)
	if !ok {
		problem(w, 401, "unauthorized", "Sign in to manage your account.", requestID(r))
		return
	}
	if !csrf(r, session.CSRF) {
		problem(w, 403, "csrf", "CSRF validation failed.", requestID(r))
		return
	}
	if !s.accountLimiter.allow(s.clientIP(r)) {
		problem(w, 429, "rate_limited", "Too many account changes. Try again later.", requestID(r))
		return
	}
	var err error
	if password {
		var input action.PasswordChange
		if !decode(w, r, &input) {
			return
		}
		err = s.Actions.ChangeAccountPassword(r.Context(), session.ID, requestID(r), input)
	} else {
		var input struct{}
		if !decode(w, r, &input) {
			return
		}
		err = s.Actions.RevokeAccountSessions(r.Context(), session.ID, requestID(r))
	}
	if err != nil {
		respondError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "bean_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.SecureCookies})
	write(w, 200, map[string]bool{"ok": true})
}
