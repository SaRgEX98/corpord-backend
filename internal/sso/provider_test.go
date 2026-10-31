package sso

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestProvidersRejectInvalidUserInfo(t *testing.T) {
	for _, providerName := range []string{"google", "yandex"} {
		t.Run(providerName, func(t *testing.T) {
			valid := `{"id":"123","default_email":"user@example.com","real_name":"User"}`
			if providerName == "google" {
				valid = `{"sub":"123","email":"user@example.com","email_verified":true,"name":"User"}`
			}
			cases := []struct {
				name      string
				status    int
				body      string
				wantError bool
			}{
				{"success", 200, valid, false},
				{"unauthorized", 401, valid, true},
				{"rate_limited", 429, valid, true},
				{"server_error", 500, valid, true},
				{"error_json", 200, `{"error":"bad token"}`, true},
				{"invalid_json", 200, `{`, true},
				{"missing_email", 200, `{"sub":"123","id":"123","email_verified":true}`, true},
				{"missing_id", 200, `{"email":"user@example.com","default_email":"user@example.com","email_verified":true}`, true},
				{"oversized", 200, strings.Repeat(" ", maxUserInfoBytes+1), true},
			}
			if providerName == "google" {
				cases = append(cases, struct {
					name      string
					status    int
					body      string
					wantError bool
				}{"unverified_email", 200, `{"sub":"123","email":"user@example.com","email_verified":false}`, true})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("Authorization") == "" {
							t.Error("provider request missing authorization")
						}
						w.WriteHeader(tc.status)
						io.WriteString(w, tc.body)
					}))
					defer server.Close()
					p := testProvider(providerName, server.URL)
					info, err := p.GetUserInfo(context.Background(), &oauth2.Token{AccessToken: "provider-access"})
					if (err != nil) != tc.wantError {
						t.Fatalf("userinfo=%+v error=%v, wantError=%v", info, err, tc.wantError)
					}
					if tc.wantError && info != nil {
						t.Fatal("provider returned a usable identity with an error")
					}
				})
			}
		})
	}
}

func testProvider(providerName, endpoint string) Provider {
	if providerName == "google" {
		p := NewGoogleProvider("client", "secret", "https://app.example/callback").(*GoogleProvider)
		p.userInfoURL = endpoint
		p.config.Endpoint.TokenURL = endpoint
		p.client.Timeout = 100 * time.Millisecond
		return p
	}
	p := NewYandexProvider("client", "secret", "https://app.example/callback").(*YandexProvider)
	p.userInfoURL = endpoint
	p.cfg.Endpoint.TokenURL = endpoint
	p.client.Timeout = 100 * time.Millisecond
	return p
}

func TestProvidersUsePKCEAndTimeout(t *testing.T) {
	for _, name := range []string{"google", "yandex"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					if r.PostForm.Get("code_verifier") != "test-code-verifier" {
						t.Error("PKCE verifier missing from exchange")
					}
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"access_token":"access","token_type":"Bearer"}`)
					return
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			p := testProvider(name, server.URL)
			loginURL, err := url.Parse(p.AuthURL("test-state", "test-code-verifier"))
			if err != nil {
				t.Fatal(err)
			}
			q := loginURL.Query()
			if q.Get("state") != "test-state" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != oauth2.S256ChallengeFromVerifier("test-code-verifier") {
				t.Fatalf("PKCE URL parameters missing: %v", q)
			}
			if _, err := p.Exchange(context.Background(), "test-code", "test-code-verifier"); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Exchange(context.Background(), "test-code", ""); err == nil {
				t.Fatal("exchange accepted missing PKCE verifier")
			}
			started := time.Now()
			if _, err := p.GetUserInfo(context.Background(), &oauth2.Token{AccessToken: "access"}); err == nil {
				t.Fatal("timeout did not fail userinfo")
			}
			if time.Since(started) > time.Second {
				t.Fatal("provider client timeout was not applied")
			}
		})
	}
}

func TestProviderExchangeErrorDoesNotExposeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"secret-code-value"}`)
	}))
	defer server.Close()
	for _, name := range []string{"google", "yandex"} {
		_, err := testProvider(name, server.URL).Exchange(context.Background(), "code", "verifier")
		if err == nil || strings.Contains(err.Error(), "secret-code-value") {
			t.Fatalf("unsafe exchange error: %v", err)
		}
	}
}
