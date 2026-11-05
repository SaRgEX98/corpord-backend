package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Only a test shop is supported. Credentials are never echoed in validation errors.
func (p Payments) Validate(appEnv string) error {
	if p.Mode != "" && p.Mode != "test" {
		return fmt.Errorf("PAYMENTS_MODE must be test; live payments are not supported")
	}
	if !p.Enabled {
		return nil
	}
	if p.Mode != "test" || p.ShopID == "" || !strings.HasPrefix(p.SecretKey, "test_") || len(p.SecretKey) <= 5 || strings.ContainsAny(p.SecretKey, "\r\n\t ") {
		return fmt.Errorf("payments require test mode, test-shop ID and test secret key")
	}
	for _, r := range p.ShopID {
		if r < '0' || r > '9' {
			return fmt.Errorf("YOOKASSA_SHOP_ID must be numeric")
		}
	}
	if p.Timeout < time.Second || p.Timeout > 15*time.Second {
		return fmt.Errorf("PAYMENTS_TIMEOUT must be between 1s and 15s")
	}
	u, err := url.Parse(p.ReturnOrigin)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Opaque != "" {
		return fmt.Errorf("PAYMENTS_RETURN_ORIGIN must be an exact HTTPS origin")
	}
	if u.Scheme == "https" {
		return nil
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if u.Scheme == "http" && (appEnv == "development" || appEnv == "test") && (host == "localhost" || ip != nil && ip.IsLoopback()) {
		return nil
	}
	return fmt.Errorf("PAYMENTS_RETURN_ORIGIN must be HTTPS, or loopback HTTP in development/test")
}
