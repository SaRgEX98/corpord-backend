//go:build checkout_e2e

package app

import (
	"context"
	"corpord-api/internal/payments"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// This controllable provider and its endpoints exist only in the explicitly
// build-tagged disposable integration binary, never cmd/server or its image.
type e2eProvider struct {
	mu        sync.Mutex
	next      int
	values    map[string]payments.Payment
	creates   map[string]string
	scenarios map[string]string
	effects   map[string]int
	calls     map[string]int
	refunds   map[string]payments.Refund
}

func newE2EProvider() *e2eProvider {
	return &e2eProvider{values: map[string]payments.Payment{}, creates: map[string]string{}, scenarios: map[string]string{}, effects: map[string]int{}, calls: map[string]int{}, refunds: map[string]payments.Refund{}}
}
func (p *e2eProvider) CreateAuthorization(_ context.Context, key string, r payments.AuthorizationRequest) (payments.Payment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls["create"]++
	if id := p.creates[key]; id != "" {
		return p.values[id], nil
	}
	p.next++
	id := fmt.Sprintf("fixture-payment-%08d", p.next)
	expiry := time.Now().Add(time.Hour)
	v := payments.Payment{ID: id, Status: "pending", AmountMinor: r.AmountMinor, Currency: r.Currency, Test: true, MerchantID: "100500", IntentID: r.IntentID, ConfirmationURL: "https://yoomoney.ru/test/" + id, ExpiresAt: &expiry}
	p.values[id] = v
	p.creates[key] = id
	p.effects["create"]++
	return v, nil
}
func (p *e2eProvider) GetPayment(_ context.Context, id string) (payments.Payment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls["get"]++
	v, ok := p.values[id]
	if !ok {
		return v, &payments.Error{Kind: payments.Rejected}
	}
	if p.scenarios[id] == "capture_unknown" && v.Status == "succeeded" {
		return payments.Payment{}, &payments.Error{Kind: payments.Unknown}
	}
	return v, nil
}
func (p *e2eProvider) Capture(_ context.Context, key, id string, amount int64) (payments.Payment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls["capture"]++
	v, ok := p.values[id]
	if !ok || amount != v.AmountMinor {
		return v, &payments.Error{Kind: payments.Security}
	}
	if v.Status == "waiting_for_capture" {
		if p.scenarios[id] == "capture_fail" {
			v.Status = "canceled"
		} else {
			v.Status = "succeeded"
			p.effects["capture"]++
		}
		p.values[id] = v
		if p.scenarios[id] == "capture_unknown" {
			return payments.Payment{}, &payments.Error{Kind: payments.Unknown}
		}
	}
	return v, nil
}
func (p *e2eProvider) CancelAuthorization(_ context.Context, key, id string) (payments.Payment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls["cancel"]++
	v := p.values[id]
	if v.Status == "waiting_for_capture" {
		v.Status = "canceled"
		p.values[id] = v
		p.effects["cancel"]++
	}
	return v, nil
}
func (p *e2eProvider) CreateRefund(_ context.Context, key string, r payments.RefundRequest) (payments.Refund, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls["refund"]++
	if v, ok := p.refunds[key]; ok {
		return v, nil
	}
	v := payments.Refund{ID: "fixture-refund-" + r.PaymentID, PaymentID: r.PaymentID, Status: "succeeded", AmountMinor: r.AmountMinor, Currency: r.Currency}
	p.refunds[key] = v
	p.effects["refund"]++
	return v, nil
}
func (p *e2eProvider) GetRefund(_ context.Context, id string) (payments.Refund, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, v := range p.refunds {
		if v.ID == id {
			return v, nil
		}
	}
	return payments.Refund{}, &payments.Error{Kind: payments.Rejected}
}
func NewCheckoutE2E() *App {
	provider := newE2EProvider()
	return newApp(provider, func(r *gin.Engine) {
		r.POST("/__test/checkout/authorize", func(c *gin.Context) {
			var body struct {
				CheckoutID int64  `json:"checkout_id"`
				Scenario   string `json:"scenario"`
			}
			if c.ShouldBindJSON(&body) != nil || body.CheckoutID < 1 {
				c.Status(400)
				return
			}
			switch body.Scenario {
			case "paid", "capture_unknown", "capture_fail", "refund":
			default:
				c.Status(400)
				return
			}
			provider.mu.Lock()
			defer provider.mu.Unlock()
			for id, v := range provider.values {
				if v.IntentID == strconv.FormatInt(body.CheckoutID, 10) {
					provider.scenarios[id] = body.Scenario
					if body.Scenario == "refund" {
						v.Status = "succeeded"
					} else {
						v.Status = "waiting_for_capture"
					}
					provider.values[id] = v
					c.JSON(200, gin.H{"payment_id": id, "status": v.Status})
					return
				}
			}
			c.Status(404)
		})

		r.POST("/__test/checkout/resolve", func(c *gin.Context) {
			var body struct {
				CheckoutID int64 `json:"checkout_id"`
			}
			if c.ShouldBindJSON(&body) != nil || body.CheckoutID < 1 {
				c.Status(400)
				return
			}
			provider.mu.Lock()
			defer provider.mu.Unlock()
			for id, v := range provider.values {
				if v.IntentID == strconv.FormatInt(body.CheckoutID, 10) {
					delete(provider.scenarios, id)
					c.JSON(200, gin.H{"payment_id": id, "status": v.Status})
					return
				}
			}
			c.Status(404)
		})
		r.GET("/__test/checkout/status", func(c *gin.Context) {
			provider.mu.Lock()
			defer provider.mu.Unlock()
			c.JSON(200, gin.H{"calls": provider.calls, "effects": provider.effects})
		})
	})
}
