package config

import (
	"strings"
	"testing"
	"time"
)

func TestPaymentsTestOnlyConfig(t *testing.T) {
	valid := Payments{Enabled: true, Mode: "test", ShopID: "100500", SecretKey: "test_fixture_only_not_a_real_key", ReturnOrigin: "https://app.example.test", Timeout: 10 * time.Second}
	if err := valid.Validate("production"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Payments){func(p *Payments) { p.Mode = "live" }, func(p *Payments) { p.ShopID = "bad" }, func(p *Payments) { p.SecretKey = "live_never_allowed" }, func(p *Payments) { p.SecretKey = "test_" }, func(p *Payments) { p.Timeout = 0 }, func(p *Payments) { p.ReturnOrigin = "https://user:password@app.example.test" }, func(p *Payments) { p.ReturnOrigin = "https://app.example.test/path" }, func(p *Payments) { p.ReturnOrigin = "https://app.example.test?token=private" }, func(p *Payments) { p.ReturnOrigin = "http://localhost:3000" }} {
		p := valid
		mutate(&p)
		err := p.Validate("production")
		if err == nil {
			t.Fatal("accepted invalid payment config")
		}
		if strings.Contains(err.Error(), p.SecretKey) || strings.Contains(err.Error(), "private") {
			t.Fatal("credentials leaked")
		}
	}
	valid.ReturnOrigin = "http://localhost:3103"
	if err := valid.Validate("development"); err != nil {
		t.Fatal(err)
	}
	valid.ReturnOrigin = "http://localhost.attacker.test"
	if err := valid.Validate("development"); err == nil {
		t.Fatal("accepted attacker origin")
	}
	if err := (Payments{}).Validate("production"); err != nil {
		t.Fatal("zero-value disabled config must remain compatible")
	}
}
func TestPaymentEnvironmentParsing(t *testing.T) {
	loadTestConfig(t)
	for key, value := range map[string]string{"PAYMENTS_ENABLED": "not-bool", "PAYMENTS_TIMEOUT": "not-duration", "PAYMENTS_MODE": "live"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid environment accepted")
			}
		})
	}
	t.Setenv("PAYMENTS_ENABLED", "true")
	t.Setenv("YOOKASSA_SHOP_ID", "100500")
	t.Setenv("YOOKASSA_SECRET_KEY", "test_fixture_only_not_real")
	t.Setenv("PAYMENTS_RETURN_ORIGIN", "http://localhost:3000")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}
