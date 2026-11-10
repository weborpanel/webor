// Package api serves the panel HTTP JSON API.
package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/webooza/webooza/internal/auth"
	"github.com/webooza/webooza/internal/dnscheck"
	"github.com/webooza/webooza/internal/helper"
)

// Server holds panel API dependencies.
type Server struct {
	Account    *auth.Account
	Issuer     string // TOTP issuer label
	ConfigPath string // desired-state YAML; empty = DefaultConfigPath
	// Ops runs privileged site filesystem ops. Lab/dev may use helper.Ops
	// directly; production should go through webooza-helper IPC later.
	Ops *helper.Ops

	// DNS is the resolver used for site checklists (tests inject Fake).
	DNS dnscheck.Resolver
	// ExpectedA / ExpectedAAAA are the VPS addresses sites should point at.
	ExpectedA    []string
	ExpectedAAAA []string

	// Caddy apply options used after site create/delete (best-effort).
	CaddyConfigPath string
	HTTPListen      string
	DisableHTTPS    bool // lab / until DNS passes
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) writeErr(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}

// MountAuth registers /api/auth/* routes on mux.
func (s *Server) MountAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/status", s.handleStatus)
	mux.HandleFunc("POST /api/auth/setup", s.handleSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("POST /api/auth/totp/begin", s.handleTOTPBegin)
	mux.HandleFunc("POST /api/auth/totp/confirm", s.handleTOTPConfirm)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	has, err := s.Account.HasAdmin()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":           true,
		"needs_setup":  !has,
		"authenticated": s.sessionOK(r) != nil,
	})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	if err := s.Account.CompleteSetup(body.Token, body.Email, body.Password); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TOTP     string `json:"totp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	tok, err := s.Account.Login(body.Email, body.Password, body.TOTP, clientIP(r), r.UserAgent())
	if err != nil {
		s.writeErr(w, 401, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().UTC().Add(auth.DefaultSessionTTL),
	})
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookie); err == nil {
		_ = s.Account.DeleteSessionByToken(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if s.sessionOK(r) == nil {
		s.writeErr(w, 401, "unauthorized")
		return
	}
	adm, err := s.Account.GetAdmin()
	if err != nil {
		s.writeErr(w, 500, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"ok":             true,
		"email":          adm.Email,
		"totp_confirmed": adm.TOTPConfirmed,
	})
}

func (s *Server) handleTOTPBegin(w http.ResponseWriter, r *http.Request) {
	if s.sessionOK(r) == nil {
		s.writeErr(w, 401, "unauthorized")
		return
	}
	issuer := s.Issuer
	if issuer == "" {
		issuer = "Webooza"
	}
	secret, url, err := s.Account.BeginTOTPEnrollment(issuer)
	if err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "secret": secret, "otpauth_url": url})
}

func (s *Server) handleTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	if s.sessionOK(r) == nil {
		s.writeErr(w, 401, "unauthorized")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeErr(w, 400, "invalid json")
		return
	}
	if err := s.Account.ConfirmTOTP(body.Code); err != nil {
		s.writeErr(w, 400, err.Error())
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) sessionOK(r *http.Request) *auth.Session {
	c, err := r.Cookie(auth.SessionCookie)
	if err != nil {
		return nil
	}
	sess, err := s.Account.LookupSession(c.Value)
	if err != nil {
		return nil
	}
	return sess
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
