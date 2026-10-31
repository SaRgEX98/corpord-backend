package sso

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

// LoginTimeout bounds exchange, userinfo and account/session persistence together.
const LoginTimeout = 10 * time.Second
const maxUserInfoBytes = 1 << 20

func providerHTTPClient() *http.Client {
	return &http.Client{Timeout: LoginTimeout}
}

func decodeUserInfo(resp *http.Response, dst any) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("userinfo returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUserInfoBytes+1))
	if err != nil {
		return errors.New("failed to read userinfo")
	}
	if len(body) > maxUserInfoBytes {
		return errors.New("userinfo response is too large")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return errors.New("invalid userinfo JSON")
	}
	return nil
}

// ValidateUserInfo rejects incomplete provider responses before any account is created.
func ValidateUserInfo(info *UserInfo) error {
	if info == nil || strings.TrimSpace(info.Provider) == "" || strings.TrimSpace(info.ProviderID) == "" {
		return errors.New("missing provider identity")
	}
	if len(info.ProviderID) > 512 {
		return errors.New("invalid provider identity")
	}
	addr, err := mail.ParseAddress(info.Email)
	if err != nil || addr.Address != info.Email || len(info.Email) > 255 {
		return errors.New("provider did not return a valid email")
	}
	if info.Provider == "google" && !info.EmailVerified {
		return errors.New("provider email is not verified")
	}
	return nil
}
