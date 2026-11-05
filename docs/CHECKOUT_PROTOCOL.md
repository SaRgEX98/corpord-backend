# Durable demo checkout (ЮKassa test shop)

This stage creates sandbox checkout for an existing **demo** seat hold. It never
activates live charging, accepts card details, creates a real carrier ticket or
uses the unfinished legacy `orders`/`payments` tables. Default capability is off.
The adapter separately enforces a test-shop key and verifies `test=true`, merchant,
card method, supported statuses and exact RUB money. Business code independently
compares payment ID, immutable intent ID, amount and currency on every fact;
a stored merchant must remain unchanged. Card amounts supported here are
100–35,000,000 kopecks. Free/sub-ruble fares require a separate product flow and
return `not_payable` without a provider call.

Public contracts: [checkout.openapi.json](checkout.openapi.json),
[booking.openapi.json](booking.openapi.json), and the frozen
[implementation contract](planning/payments-v1/CONTRACTS.md).
Provider-only behavior: [YOOKASSA_ADAPTER.md](YOOKASSA_ADAPTER.md).

## One seat ledger

Migration `20261105000300` extends existing `booking_holds` instead of creating
an independent paid-seat table. Exactly one GiST exclusion covers the same
trip/seat/half-open section interval in `held`, `payment_committing`, `confirmed`.
A held allocation occupies only until its original DB-clock expiry. The other
two occupy independently of that expiry; the normal booking expiry worker only
changes `held`. Allocators, availability, hold limit and legacy cancel paths use
the expanded model. Paid allocations can share seats on adjacent sections;
overlapping sections cannot. Checkout capability and nullable `checkout_id` are
returned directly on every Booking response without per-row fetches.

Trip/timetable edits protect committing/confirmed history. Bus shrinking or
soft deletion is refused for committing allocations and future confirmed trips.
After a confirmed trip has finished, that reusable bus may change capacity;
the immutable historical allocation remains in its own trip ledger. Effective
inventory remains `min(snapshot, current bus capacity)` and never grows silently.

Every business transition locks owner → trip/bus → hold → order/attempt. Generic
timetable writers lock the same trip. Owner locking also serializes create/cancel
and the five-active-hold limit. Background recovery locks the owner even after
soft deletion, so disabling an account cannot strand a payment. Interactive
mutations keep the active-account check. Financial history prevents physical
account/trip deletion by foreign keys.

## Checkout and payment are separate state machines

Create validates an own live hold and copies immutable passenger, amount, route,
timezone and seat snapshots. There is one order per hold and one provider attempt
per order. Owner/idempotency-key uniqueness makes double clicks return the same
intent; a key for a different hold returns409. No retry renews the initial TTL.
Provider metadata intent is the decimal local order ID. Separate persisted UUID
keys exist before any create/capture/cancel/refund call.

1. Transaction commits order/attempt and create outbox, then worker creates hosted
   authorization with `capture=false`. Network always runs outside SQL locks.
2. A verified `waiting_for_capture` may atomically change the still-live allocation
   to `payment_committing` and enqueue capture. It must beat expiry/cancel and
   departure. A pending payment does not extend the hold.
3. Capture runs after commit. Verified success confirms the protected seat and
   marks the order paid. A lost response schedules a GET reconciliation and keeps
   the committing allocation; a local timeout is never proof of cancellation.
4. If cancel/expiry wins first, a later authorization is cancelled without capture.
   Provider pending cannot be assumed cancelled; GET polling continues until a
   terminal fact or authorization that can actually be cancelled.
5. New capture dispatch after departure is stopped. Verified late/unexpected
   success without valid committing fulfillment creates a durable full refund.
   Replays share one refund key; lost refund responses replay that key. Unknown or
   failed compensation remains visible and preserves a committing allocation.
6. Verified paid/refunded states do not regress on older pending/cancel events.
   A terminal failed refund stays in review when older pending/success hints recur;
   verified refund success may still resolve it. User partial refunds/paid-trip
   cancellation are outside this stage.

Legacy hold cancel rejects any nonterminal linked checkout, and always rejects
committing/confirmed release. Checkout cancel before committing persists cancel
intent and releases the held seat; after committing/paid it returns409. Returns
from hosted payment and client query strings never establish payment success.

## Durable jobs, notifications and recovery

Outbox jobs and inbox events use PostgreSQL pending/leased/done states, short
30-second leases, `SKIP LOCKED`, and an incrementing fencing token. Publishing a
response requires the current unexpired lease under the same business locks.
A stale worker cannot publish a result after takeover. Provider idempotency keys,
not a lease timeout, make replay of an uncertain network effect safe.

A worker claim counts one work item against the requested batch (1–1000). It
claims one item at a time, interleaves outbox and inbox, and respects caller context;
root schedules a bounded batch with its own timeout and graceful cancellation.
Network failures use exponential backoff and a separate consecutive-failure
budget of20. Successful pending reads do not consume this failure budget. Pending
polling reschedules in the future; resurrecting a completed job resets its due
time, avoiding a burst of repeated GETs within the same worker batch.

An unknown create/capture/refund is never blindly replayed beyond23 hours from
the durable operation job, below the provider's finite idempotency window. It
becomes `review_required`; uncertain committing seats stay protected. Operators
must reconcile outside this automatic retry budget; restarting the process does
not generate new operation keys or silently treat unknown money as unpaid.

Webhook body is a hint (`event`, opaque object ID), not payment evidence. Intake
persists only IDs already indexed as our payment/refund and acknowledges after
that statement commits. Arbitrary unknown IDs are acknowledged and ignored:
there is no unbounded foreign-ID storage or authenticated provider lookup proxy.
A webhook that arrives before a lost create response is recovered by replaying
the original create key and the existing reconciliation job. Known notifications
are verified with provider GET; the target order is selected by the persisted
provider/refund ID, never redirected by untrusted/mismatching metadata. Worker
retains a quarantine fallback if a previously known mapping disappears.

## Reading and rollout

Own checkout list/detail read snapshots only, default30/max100, ID keysets scoped
to owner and status, one SQL regardless of1/30/100 rows. No provider calls, total
COUNT or per-row passenger/trip fetches. Detail alone exposes a verified hosted
HTTPS URL when server actions permit it; private routes set `Cache-Control:
no-store`. URLs, names, provider payloads, operation keys and secrets are not logged.
Reconcile is a five-second per-order cooldown that wakes existing durable work;
it never creates a new attempt or payment key. Disabled mode rejects new payment
intents and leaves existing records readable and protected.

Stop the old API and old workers for the local upgrade, apply migration, then
start the matching new code. Do not mix seat readers/writers from different
protocol versions. The migration preserves existing holds. Down refuses any
checkout/inbox history or protected allocation instead of silently deleting
financial data. A rollback therefore requires an explicit reconciliation/archive
procedure; no downgrade into a running old checkout worker is supported.

## Verification boundary

All mutation tests use explicitly supplied disposable `TEST_DATABASE_URL`, random
schemas and synthetic accounts. Go race tests cover concurrent intent creation,
all seat states and adjacency, owner/cursor isolation, exact money comparisons,
pending-poll bounds, cancel/expiry/departure, unknown capture and lost success,
lease takeover/stale response, soft-deleted account recovery, mismatching facts,
lost create/refund and one compensation effect, failed-refund monotonicity,
provider idempotency-window cutoff, existing-hold migration/down refusal,
1/30/100-row SQL counts and guarded bus reuse. Handler tests cover strict bodies,
capability off, auth, safe errors, cooldown headers and no-store.

These fake-provider/HTTP-adapter checks do not prove an actual ЮKassa test-shop
round trip. Real sandbox credentials, notification reachability and shop settings
must be configured separately through server secret storage. Live payment,
fiscal receipt/taxes, real inventory, true tickets, partial refunds, notifications,
retention and an operator review UI remain outside this implementation.
