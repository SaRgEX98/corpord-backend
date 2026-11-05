package checkout

import (
	"context"
	"corpord-api/internal/payments"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// A lease authorizes one worker generation to publish a result, not to assume
// money was not moved. Operation keys survive every retry and lease takeover.
type work struct {
	ID             int64     `db:"id"`
	OrderID        int64     `db:"order_id"`
	Kind           string    `db:"kind"`
	ObjectID       string    `db:"object_id"`
	Attempts       int       `db:"attempts"`
	Failures       int       `db:"failures"`
	NetworkStarted bool      `db:"network_started"`
	Fence          int64     `db:"fence"`
	CreatedAt      time.Time `db:"created_at"`
	Inbox          bool      `db:"-"`
}

func (s *Service) claim(ctx context.Context, inbox bool) (work, bool, error) {
	var w work
	w.Inbox = inbox
	table := "checkout_jobs"
	projection := `j.id,j.order_id,j.kind,j.attempts,j.failures,j.network_started,j.fence,j.created_at`
	if inbox {
		table = "checkout_inbox"
		projection = `j.id,j.event kind,j.object_id,j.attempts,j.failures,j.fence,j.created_at`
	}
	q := `WITH candidate AS (SELECT id FROM ` + table + ` WHERE (state='pending' AND available_at<=clock_timestamp()) OR (state='leased' AND lease_until<=clock_timestamp()) ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE ` + table + ` j SET state='leased',fence=j.fence+1,attempts=j.attempts+1,lease_until=clock_timestamp()+interval '30 seconds' FROM candidate c WHERE j.id=c.id RETURNING ` + projection
	err := s.DB.GetContext(ctx, &w, q)
	if errors.Is(err, sql.ErrNoRows) {
		return w, false, nil
	}
	return w, err == nil, err
}
func tableFor(w work) string {
	if w.Inbox {
		return "checkout_inbox"
	}
	return "checkout_jobs"
}
func lockWork(ctx context.Context, tx *sqlx.Tx, w work) (bool, error) {
	var id int64
	err := tx.GetContext(ctx, &id, `SELECT id FROM `+tableFor(w)+` WHERE id=$1 AND fence=$2 AND state='leased' AND lease_until>clock_timestamp() FOR UPDATE`, w.ID, w.Fence)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func finish(ctx context.Context, tx *sqlx.Tx, w work) error {
	_, err := tx.ExecContext(ctx, `UPDATE `+tableFor(w)+` SET state='done',lease_until=NULL,last_error=NULL,failures=0 WHERE id=$1 AND fence=$2`, w.ID, w.Fence)
	return err
}
func (s *Service) Worker(ctx context.Context, limit int) error {
	if limit < 1 || limit > 1000 {
		return failure(400, "invalid_request")
	}
	if !s.options.Enabled || s.provider == nil {
		return nil
	}
	for n := 0; n < limit; n++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		w, found, err := s.claim(ctx, n%2 == 1)
		if err != nil {
			return err
		}
		if !found {
			w, found, err = s.claim(ctx, n%2 != 1)
			if err != nil {
				return err
			}
			if !found {
				return nil
			}
		}
		if w.Inbox {
			err = s.processInbox(ctx, w)
		} else {
			err = s.processJob(ctx, w)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) preflight(ctx context.Context, w work) (record, string, bool, error) {
	var empty record
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return empty, "", false, err
	}
	defer tx.Rollback()
	r, err := lockOrder(ctx, tx, 0, w.OrderID)
	if err != nil {
		return empty, "", false, err
	}
	valid, err := lockWork(ctx, tx, w)
	if err != nil || !valid {
		return empty, "", false, err
	}
	done := func() (record, string, bool, error) {
		if err := finish(ctx, tx, w); err != nil {
			return empty, "", false, err
		}
		return r, "", false, tx.Commit()
	}
	if r.Status == "paid" || r.Status == "refunded" {
		return done()
	}
	if w.Kind == "create" && r.ProviderID != nil {
		if err = enqueue(ctx, tx, r.ID, "reconcile", 0); err != nil {
			return empty, "", false, err
		}
		return done()
	}
	live := r.HoldStatus == "held" && r.HoldExpiresAt.After(r.Now) && r.DepartureTime.After(r.Now)
	if w.Kind == "create" && !w.NetworkStarted && (!live || r.CancelRequested || r.Status != "awaiting_payment") {
		status := "expired"
		if r.CancelRequested {
			status = "cancelled"
		}
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status=$2,updated_at=clock_timestamp() WHERE id=$1`, r.ID, status); err != nil {
			return empty, "", false, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status=CASE WHEN expires_at<=clock_timestamp() THEN 'expired' ELSE 'cancelled' END WHERE id=$1 AND status='held'`, r.HoldID); err != nil {
			return empty, "", false, err
		}
		return done()
	}
	action := w.Kind
	if w.Kind != "create" && w.Kind != "refund" && w.Kind != "reconcile_refund" && r.ProviderID == nil {
		if err = enqueue(ctx, tx, r.ID, "create", 0); err != nil {
			return empty, "", false, err
		}
		return done()
	}
	if w.Kind == "capture" {
		if r.HoldStatus != "payment_committing" || r.Status != "committing" {
			if err = enqueue(ctx, tx, r.ID, "reconcile", 0); err != nil {
				return empty, "", false, err
			}
			return done()
		}
		if !r.DepartureTime.After(r.Now) {
			if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET cancel_requested=true,status='cancel_pending',updated_at=clock_timestamp() WHERE id=$1`, r.ID); err != nil {
				return empty, "", false, err
			}
			action = "reconcile"
		}
	}
	if w.Kind == "refund" && r.RefundID != nil {
		action = "reconcile_refund"
	}
	if w.Kind == "reconcile_refund" && r.RefundID == nil {
		if err = enqueue(ctx, tx, r.ID, "refund", 0); err != nil {
			return empty, "", false, err
		}
		return done()
	}
	// YooKassa guarantees idempotence for a finite window. Never send a fresh
	// create/capture/refund after that window to resolve an unknown outcome.
	if (action == "create" || action == "capture" || action == "refund") && w.NetworkStarted && r.Now.Sub(w.CreatedAt) > 23*time.Hour {
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status='review_required',payment_status='unknown',updated_at=clock_timestamp() WHERE id=$1 AND status NOT IN ('paid','refunded')`, r.ID); err != nil {
			return empty, "", false, err
		}
		return done()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE checkout_jobs SET network_started=true WHERE id=$1 AND fence=$2`, w.ID, w.Fence); err != nil {
		return empty, "", false, err
	}
	return r, action, true, tx.Commit()
}
func (s *Service) processJob(ctx context.Context, w work) error {
	r, kind, run, err := s.preflight(ctx, w)
	if err != nil || !run {
		return err
	}
	var p payments.Payment
	var refund payments.Refund
	switch kind {
	case "create":
		p, err = s.provider.CreateAuthorization(ctx, r.CreateKey, payments.AuthorizationRequest{AmountMinor: r.AmountMinor, Currency: r.Currency, Description: fmt.Sprintf("Corpord sandbox order %d", r.ID), IntentID: r.IntentID, ReturnURL: strings.TrimRight(s.options.ReturnOrigin, "/") + fmt.Sprintf("/me/orders/%d/payment-return", r.ID)})
	case "reconcile":
		p, err = s.provider.GetPayment(ctx, *r.ProviderID)
	case "capture":
		p, err = s.provider.Capture(ctx, r.CaptureKey, *r.ProviderID, r.AmountMinor)
	case "cancel":
		p, err = s.provider.CancelAuthorization(ctx, r.CancelKey, *r.ProviderID)
	case "refund":
		refund, err = s.provider.CreateRefund(ctx, r.RefundKey, payments.RefundRequest{PaymentID: *r.ProviderID, AmountMinor: r.AmountMinor, Currency: r.Currency, IntentID: r.IntentID})
	case "reconcile_refund":
		refund, err = s.provider.GetRefund(ctx, *r.RefundID)
	default:
		return s.fail(ctx, w, &payments.Error{Kind: payments.Security, Code: "invalid_operation"})
	}
	if err != nil {
		return s.fail(ctx, w, err)
	}
	if kind == "refund" || kind == "reconcile_refund" {
		p, err = s.provider.GetPayment(ctx, refund.PaymentID)
		if err != nil {
			return s.fail(ctx, w, err)
		}
		return s.applyRefund(ctx, w, r.ID, p, refund)
	}
	return s.applyPayment(ctx, w, r.ID, p)
}
func (s *Service) processInbox(ctx context.Context, w work) error {
	var id int64
	column := "provider_id"
	if w.Kind == "refund.succeeded" {
		column = "refund_id"
	}
	err := s.DB.GetContext(ctx, &id, `SELECT order_id FROM checkout_attempts WHERE `+column+`=$1`, w.ObjectID)
	if errors.Is(err, sql.ErrNoRows) {
		return s.finishUnmatched(ctx, w)
	}
	if err != nil {
		return err
	}
	w.OrderID = id
	var p payments.Payment
	var refund payments.Refund
	if w.Kind == "refund.succeeded" {
		refund, err = s.provider.GetRefund(ctx, w.ObjectID)
		if err == nil {
			p, err = s.provider.GetPayment(ctx, refund.PaymentID)
		}
	} else {
		p, err = s.provider.GetPayment(ctx, w.ObjectID)
	}
	if err != nil {
		return s.fail(ctx, w, err)
	}
	if (w.Kind == "refund.succeeded" && refund.ID != w.ObjectID) || (w.Kind != "refund.succeeded" && p.ID != w.ObjectID) {
		return s.fail(ctx, w, &payments.Error{Kind: payments.Security, Code: "notification_mismatch"})
	}
	if w.Kind == "refund.succeeded" {
		return s.applyRefund(ctx, w, id, p, refund)
	}
	return s.applyPayment(ctx, w, id, p)
}
func (s *Service) finishUnmatched(ctx context.Context, w work) error {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	valid, err := lockWork(ctx, tx, w)
	if err != nil || !valid {
		return err
	}
	if err = finish(ctx, tx, w); err != nil {
		return err
	}
	return tx.Commit()
}
func validPayment(r record, p payments.Payment) bool {
	return p.ID != "" && len(p.ID) <= 128 && p.Test && p.MerchantID != "" && p.IntentID == r.IntentID && p.AmountMinor == r.AmountMinor && p.Currency == r.Currency && (r.ProviderID == nil || *r.ProviderID == p.ID) && (r.MerchantID == nil || *r.MerchantID == p.MerchantID) && (p.Status == "pending" || p.Status == "waiting_for_capture" || p.Status == "succeeded" || p.Status == "canceled") && (p.ConfirmationURL == "" || validConfirmation(p.ConfirmationURL))
}
func markReview(ctx context.Context, tx *sqlx.Tx, r record) error {
	_, err := tx.ExecContext(ctx, `UPDATE checkout_orders SET status='review_required',payment_status='unknown',updated_at=clock_timestamp() WHERE id=$1 AND status NOT IN ('paid','refunded')`, r.ID)
	return err
}
func (s *Service) applyPayment(ctx context.Context, w work, id int64, p payments.Payment) error {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := lockOrder(ctx, tx, 0, id)
	if err != nil {
		return err
	}
	valid, err := lockWork(ctx, tx, w)
	if err != nil || !valid {
		return err
	}
	if err = finish(ctx, tx, w); err != nil {
		return err
	}
	if !validPayment(r, p) {
		if err = markReview(ctx, tx, r); err != nil {
			return err
		}
		return tx.Commit()
	}
	var foreign bool
	if err = tx.GetContext(ctx, &foreign, `SELECT EXISTS(SELECT 1 FROM checkout_attempts WHERE provider_id=$1 AND order_id<>$2)`, p.ID, r.ID); err != nil {
		return err
	}
	if foreign {
		if err = markReview(ctx, tx, r); err != nil {
			return err
		}
		return tx.Commit()
	}
	// A delayed pending/authorization notification cannot roll back verified money.
	if r.ProviderStatus != nil && ((*r.ProviderStatus == "succeeded" && p.Status != "succeeded") || (*r.ProviderStatus == "canceled" && (p.Status == "pending" || p.Status == "waiting_for_capture"))) {
		return tx.Commit()
	}
	if r.Status == "refunded" || r.Status == "paid" {
		return tx.Commit()
	}
	if r.RefundStatus != nil && *r.RefundStatus == "canceled" && p.Status == "succeeded" {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE checkout_attempts SET provider_id=$2,merchant_id=$3,provider_status=$4,confirmation_url=NULLIF($5,'') WHERE order_id=$1`, r.ID, p.ID, p.MerchantID, p.Status, p.ConfirmationURL); err != nil {
		return err
	}
	switch p.Status {
	case "pending":
		if !r.HoldExpiresAt.After(r.Now) && r.HoldStatus == "held" {
			if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status='expired' WHERE id=$1 AND status='held'`, r.HoldID); err != nil {
				return err
			}
			if r.Status == "awaiting_payment" {
				if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status='expired' WHERE id=$1`, r.ID); err != nil {
					return err
				}
			}
		}
		ps := "pending"
		if r.HoldStatus == "payment_committing" {
			ps = "unknown"
		}
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET payment_status=$2,updated_at=clock_timestamp() WHERE id=$1 AND status NOT IN ('refund_pending','review_required')`, r.ID, ps); err != nil {
			return err
		}
		if err = enqueue(ctx, tx, r.ID, "reconcile", 3*time.Second); err != nil {
			return err
		}
	case "waiting_for_capture":
		live := r.HoldStatus == "held" && r.HoldExpiresAt.After(r.Now) && r.DepartureTime.After(r.Now) && !r.CancelRequested && r.Status == "awaiting_payment"
		committing := r.HoldStatus == "payment_committing" && r.Status == "committing" && !r.CancelRequested && r.DepartureTime.After(r.Now)
		if live || committing {
			if live {
				if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status='payment_committing' WHERE id=$1 AND status='held'`, r.HoldID); err != nil {
					return err
				}
			}
			if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status='committing',payment_status='processing',updated_at=clock_timestamp() WHERE id=$1`, r.ID); err != nil {
				return err
			}
			if err = enqueue(ctx, tx, r.ID, "capture", 0); err != nil {
				return err
			}
		} else {
			if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status=CASE WHEN expires_at<=$2 THEN 'expired' ELSE 'cancelled' END WHERE id=$1 AND status='held'`, r.HoldID, r.Now); err != nil {
				return err
			}
			status := "expired"
			if r.CancelRequested || r.HoldStatus == "payment_committing" {
				status = "cancel_pending"
			}
			if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status=$2,payment_status='authorized',updated_at=clock_timestamp() WHERE id=$1 AND status NOT IN ('refund_pending')`, r.ID, status); err != nil {
				return err
			}
			if err = enqueue(ctx, tx, r.ID, "cancel", 0); err != nil {
				return err
			}
		}
	case "succeeded":
		if r.HoldStatus == "payment_committing" && r.Status == "committing" && r.DepartureTime.After(r.Now) && !r.CancelRequested {
			if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status='confirmed' WHERE id=$1 AND status='payment_committing'`, r.HoldID); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status='paid',payment_status='succeeded',updated_at=clock_timestamp() WHERE id=$1`, r.ID); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE checkout_jobs SET state='done',lease_until=NULL WHERE order_id=$1 AND kind NOT IN ('refund','reconcile_refund')`, r.ID); err != nil {
				return err
			}
		} else {
			if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status='refund_pending',payment_status='refunding',updated_at=clock_timestamp() WHERE id=$1`, r.ID); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status='cancelled',cancelled_at=$2 WHERE id=$1 AND status='held'`, r.HoldID, r.Now); err != nil {
				return err
			}
			if err = enqueue(ctx, tx, r.ID, "refund", 0); err != nil {
				return err
			}
		}
	case "canceled":
		if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status=CASE WHEN expires_at<=$2 THEN 'expired' ELSE 'cancelled' END,cancelled_at=CASE WHEN expires_at>$2 THEN $2 ELSE cancelled_at END WHERE id=$1 AND status IN ('held','payment_committing')`, r.HoldID, r.Now); err != nil {
			return err
		}
		status := "cancelled"
		if !r.HoldExpiresAt.After(r.Now) && !r.CancelRequested {
			status = "expired"
		}
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status=$2,payment_status='canceled',updated_at=clock_timestamp() WHERE id=$1`, r.ID, status); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_jobs SET state='done',lease_until=NULL WHERE order_id=$1 AND kind NOT IN ('refund','reconcile_refund')`, r.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Service) applyRefund(ctx context.Context, w work, id int64, p payments.Payment, f payments.Refund) error {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := lockOrder(ctx, tx, 0, id)
	if err != nil {
		return err
	}
	valid, err := lockWork(ctx, tx, w)
	if err != nil || !valid {
		return err
	}
	if err = finish(ctx, tx, w); err != nil {
		return err
	}
	good := validPayment(r, p) && p.Status == "succeeded" && r.ProviderID != nil && f.ID != "" && f.PaymentID == *r.ProviderID && f.AmountMinor == r.AmountMinor && f.Currency == r.Currency && (r.RefundID == nil || *r.RefundID == f.ID) && (f.Status == "pending" || f.Status == "succeeded" || f.Status == "canceled")
	if !good {
		if err = markReview(ctx, tx, r); err != nil {
			return err
		}
		return tx.Commit()
	}
	if r.Status == "refunded" || (r.RefundStatus != nil && *r.RefundStatus == "canceled" && f.Status == "pending") {
		return tx.Commit()
	}
	var foreign bool
	if err = tx.GetContext(ctx, &foreign, `SELECT EXISTS(SELECT 1 FROM checkout_attempts WHERE refund_id=$1 AND order_id<>$2)`, f.ID, r.ID); err != nil {
		return err
	}
	if foreign {
		if err = markReview(ctx, tx, r); err != nil {
			return err
		}
		return tx.Commit()
	}
	// A provider-side refund can arrive before our POST response. Fulfillment is
	// withdrawn only after verification; old payment success cannot undo this.
	if _, err = tx.ExecContext(ctx, `UPDATE checkout_attempts SET refund_id=$2,refund_status=$3 WHERE order_id=$1`, r.ID, f.ID, f.Status); err != nil {
		return err
	}
	switch f.Status {
	case "succeeded":
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status='refunded',payment_status='refunded',updated_at=clock_timestamp() WHERE id=$1`, r.ID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status=CASE WHEN expires_at<=$2 THEN 'expired' ELSE 'cancelled' END,cancelled_at=CASE WHEN expires_at>$2 THEN $2 ELSE cancelled_at END WHERE id=$1 AND status IN ('held','payment_committing','confirmed')`, r.HoldID, r.Now); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_jobs SET state='done',lease_until=NULL WHERE order_id=$1`, r.ID); err != nil {
			return err
		}
	case "pending":
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status='refund_pending',payment_status='refunding',updated_at=clock_timestamp() WHERE id=$1`, r.ID); err != nil {
			return err
		}
		if err = enqueue(ctx, tx, r.ID, "reconcile_refund", 3*time.Second); err != nil {
			return err
		}
	case "canceled":
		if err = markReview(ctx, tx, r); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Service) fail(ctx context.Context, w work, cause error) error {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var r record
	if !w.Inbox || w.OrderID > 0 {
		r, err = lockOrder(ctx, tx, 0, w.OrderID)
		if err != nil {
			return err
		}
	}
	valid, err := lockWork(ctx, tx, w)
	if err != nil || !valid {
		return err
	}
	kind := payments.Unknown
	retryAfter := time.Duration(0)
	var pe *payments.Error
	if errors.As(cause, &pe) {
		kind = pe.Kind
		retryAfter = pe.RetryAfter
	}
	permanent := kind == payments.Security || kind == payments.Configuration || w.Failures >= 19
	if kind == payments.Rejected && w.Kind == "create" {
		permanent = true
	}
	if permanent {
		if err = finish(ctx, tx, w); err != nil {
			return err
		}
		if !w.Inbox || w.OrderID > 0 {
			if err = markReview(ctx, tx, r); err != nil {
				return err
			}
		}
	} else {
		delay := time.Second * time.Duration(1<<min(w.Failures+1, 6))
		if retryAfter > delay {
			delay = retryAfter
		}
		if delay > time.Hour {
			delay = time.Hour
		}
		if _, err = tx.ExecContext(ctx, `UPDATE `+tableFor(w)+` SET state='pending',lease_until=NULL,failures=failures+1,available_at=clock_timestamp()+($3*interval '1 millisecond'),last_error=$4 WHERE id=$1 AND fence=$2`, w.ID, w.Fence, delay.Milliseconds(), kind); err != nil {
			return err
		}
		if !w.Inbox || w.OrderID > 0 {
			if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET payment_status='unknown',updated_at=clock_timestamp() WHERE id=$1 AND status NOT IN ('paid','refunded')`, r.ID); err != nil {
				return err
			}
			if (w.Kind == "capture" || w.Kind == "cancel") && r.ProviderID != nil {
				if _, err = tx.ExecContext(ctx, `UPDATE checkout_jobs SET state='done',lease_until=NULL WHERE id=$1 AND fence=$2`, w.ID, w.Fence); err != nil {
					return err
				}
				if err = enqueue(ctx, tx, r.ID, "reconcile", delay); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
