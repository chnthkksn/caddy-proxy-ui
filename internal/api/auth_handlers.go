package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"caddy-proxy-ui/internal/qr"
	"caddy-proxy-ui/internal/service"
)

type setupRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if req.Password != req.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "passwords do not match")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	if err := s.auth.Setup(req.Username, req.Password); err != nil {
		if errors.Is(err, service.ErrSetupAlreadyDone) {
			writeError(w, http.StatusConflict, "an administrator account already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create administrator account")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := s.auth.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if result.TOTPChallenge != "" {
		writeJSON(w, http.StatusOK, loginResponse{TOTPRequired: true, Challenge: result.TOTPChallenge})
		return
	}
	s.setSessionCookie(w, result.Token)
	writeJSON(w, http.StatusOK, loginResponse{OK: true})
}

type loginResponse struct {
	OK           bool   `json:"ok"`
	TOTPRequired bool   `json:"totp_required,omitempty"`
	Challenge    string `json:"challenge,omitempty"`
}

type loginTOTPRequest struct {
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
}

func (s *Server) handleLoginTOTP(w http.ResponseWriter, r *http.Request) {
	var req loginTOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	token, err := s.auth.CompleteTOTPLogin(req.Challenge, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidTOTPCode):
			writeError(w, http.StatusUnauthorized, "invalid authentication code")
		case errors.Is(err, service.ErrTOTPLocked):
			writeError(w, http.StatusTooManyRequests, err.Error())
		case errors.Is(err, service.ErrTOTPChallengeInvalid):
			// 410 tells the UI to drop back to the password step.
			writeError(w, http.StatusGone, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to sign in")
		}
		return
	}

	s.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, loginResponse{OK: true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = s.auth.Logout(cookie.Value)
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "new passwords do not match")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	if err := s.auth.ChangePassword(req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, service.ErrCurrentPasswordIncorrect) {
			writeError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to change password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type totpSetupResponse struct {
	Secret string  `json:"secret"`
	URI    string  `json:"uri"`
	QR     *qrCode `json:"qr,omitempty"`
}

// qrCode is the otpauth URI as an SVG path: dark modules on a
// Size×Size viewBox that already includes the quiet zone.
type qrCode struct {
	Size int    `json:"size"`
	Path string `json:"path"`
}

type totpSetupRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	var req totpSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	secret, uri, err := s.auth.BeginTOTPSetup(req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCurrentPasswordIncorrect):
			writeError(w, http.StatusUnauthorized, "password is incorrect")
		case errors.Is(err, service.ErrTOTPAlreadyEnabled):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to start two-factor setup")
		}
		return
	}
	resp := totpSetupResponse{Secret: secret, URI: uri}
	// Only fails for a URI too long to encode (a very long username); the
	// key and link still work, so carry on without the code.
	if code, err := qr.Encode([]byte(uri)); err == nil {
		resp.QR = &qrCode{Size: code.Size + 8, Path: code.SVGPath()}
	}
	writeJSON(w, http.StatusOK, resp)
}

type totpEnableRequest struct {
	Code string `json:"code"`
}

func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	var req totpEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// requireAuth already checked the cookie is there and valid.
	cookie, _ := r.Cookie(sessionCookieName)
	if err := s.auth.EnableTOTP(req.Code, cookie.Value); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidTOTPCode):
			writeError(w, http.StatusBadRequest, "that code didn't match — check your device's clock and try again")
		case errors.Is(err, service.ErrTOTPAlreadyEnabled), errors.Is(err, service.ErrTOTPSetupNotStarted):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to enable two-factor authentication")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type totpDisableRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	var req totpDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.auth.DisableTOTP(req.Password, req.Code); err != nil {
		switch {
		case errors.Is(err, service.ErrCurrentPasswordIncorrect):
			writeError(w, http.StatusUnauthorized, "password is incorrect")
		case errors.Is(err, service.ErrInvalidTOTPCode):
			writeError(w, http.StatusUnauthorized, "invalid authentication code")
		case errors.Is(err, service.ErrTOTPLocked):
			writeError(w, http.StatusTooManyRequests, err.Error())
		case errors.Is(err, service.ErrTOTPNotEnabled):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to disable two-factor authentication")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
