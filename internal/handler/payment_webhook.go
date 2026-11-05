package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type paymentInbox interface {
	IngestWebhook(context.Context, string, string) error
}

// Only whitelisted identifiers are persisted; raw payment/card/customer objects
// never enter logs or storage. The inbox worker verifies facts through the provider.
type PaymentWebhook struct {
	inbox    paymentInbox
	enabled  bool
	mu       sync.Mutex
	window   time.Time
	requests int
}

func NewPaymentWebhook(inbox paymentInbox, enabled bool) *PaymentWebhook {
	return &PaymentWebhook{inbox: inbox, enabled: enabled}
}
func (h *PaymentWebhook) Register(public *gin.RouterGroup) {
	public.POST("/payments/yookassa/webhook", h.Receive)
}

var providerObjectID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

func (h *PaymentWebhook) allowed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if now.Sub(h.window) >= time.Minute {
		h.window = now
		h.requests = 0
	}
	if h.requests >= 120 {
		return false
	}
	h.requests++
	return true
}
func (h *PaymentWebhook) Receive(c *gin.Context) {
	if !h.enabled || h.inbox == nil {
		c.JSON(503, gin.H{"error": "Тестовая оплата отключена", "code": "payments_disabled"})
		return
	}
	if !h.allowed() {
		c.Header("Retry-After", "60")
		c.JSON(429, gin.H{"error": "Слишком много уведомлений", "code": "rate_limited"})
		return
	}
	var body struct {
		Type   string `json:"type"`
		Event  string `json:"event"`
		Object struct {
			ID string `json:"id"`
		} `json:"object"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	decoder := json.NewDecoder(c.Request.Body)
	if decoder.Decode(&body) != nil || body.Type != "notification" || !providerObjectID.MatchString(body.Object.ID) {
		c.JSON(400, gin.H{"error": "Некорректное уведомление", "code": "invalid_request"})
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		c.JSON(400, gin.H{"error": "Некорректное уведомление", "code": "invalid_request"})
		return
	}
	switch body.Event {
	case "payment.waiting_for_capture", "payment.succeeded", "payment.canceled", "refund.succeeded":
	default:
		c.JSON(200, gin.H{"status": "ignored"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if err := h.inbox.IngestWebhook(ctx, body.Event, body.Object.ID); err != nil {
		code := "inbox_unavailable"
		if errors.Is(err, context.DeadlineExceeded) {
			code = "inbox_timeout"
		}
		c.JSON(503, gin.H{"error": "Уведомление не сохранено. Повторите доставку", "code": code})
		return
	}
	c.JSON(200, gin.H{"status": "accepted"})
}
