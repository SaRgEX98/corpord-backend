# Demo seat holds and passenger workspace

Implemented 2026-11-05. The initial ten-minute **hold** can now be linked to a
sandbox checkout; see [CHECKOUT_PROTOCOL.md](CHECKOUT_PROTOCOL.md). New blocking
states `payment_committing`/`confirmed` survive the original hold TTL under the
same seat exclusion. Booking responses add `checkout_id` and `checkout_enabled`;
legacy cancellation cannot release an in-progress or confirmed checkout.
Legacy `orders` and `payments` are not used. This is not a real carrier ticket. No passport, date of
birth or payment data is collected. All new writes require the existing access
JWT; every owner predicate comes from middleware, independently of cursors.

## Storage and concurrency

Migration `20261105000100` creates saved passenger names, lazy trip inventory and
hold snapshots. Inventory is created only by the first successful hold on a demo
trip, using a snapshot of its bus capacity (supported 1–1000). The effective
capacity is the smaller of that snapshot and the current bus capacity; increasing
a bus card never silently grows an existing inventory. Public availability
is a read-only repeatable-read transaction and does not create inventory. For real
trips, capacity is not evidence of carrier stock: `status=unknown`, `seats=null`,
`bookable=false`. Search, calendar and quote keep their existing contracts.

Each hold reserves one automatically selected seat on the half-open interval
`[boarding_order, alighting_order)`. Adjacent intervals can share a seat; overlapping
ones cannot. The PostgreSQL GiST exclusion constraint enforces this independently
of application code. `btree_gist` is installed in public and retained on rollback
because it is a shared database extension. Migration privileges must permit its
installation (or an administrator must preinstall it).

Hold creation locks the user row first, then the trip and its bus, in that order.
The user lock serializes the maximum of five active holds across different trips.
The trip lock serializes seat assignment and timetable snapshot reads. The
`booking_guard_timetable` trigger takes the same parent trip lock before timetable
writes and refuses changes while an unexpired hold exists. Material trip updates
(bus, driver, times, status, base fare, demo flag) and deletion are also blocked
while held. The bus guard refuses capacity decreases, soft deletion and physical
deletion while affected active holds exist. Bus increases are allowed; after all
holds expire/cancel, a decrease constrains the next allocation even when the old
inventory snapshot is larger. Bus guard reads do not acquire a trip lock: the bus
writer lock conflicts with allocation's SHARE, preserving trip→bus lock ordering.
Trigger names order
that parent lock before the catalog-quality stop lock. A concurrent writer that
has already acquired the trip lock commits before a hold reads status, relations
and fare. Saved-passenger mutations take the owner lock as well.

Price is the exact integer sum of known nonnegative leg fares, read inside that
transaction. Unknown fare/timezone and ambiguous repeated-stop relation pairs
are not bookable. Snapshot boarding/arrival names, timestamps, timezones, passenger
name and amount remain unchanged by later template or catalogue edits. Removing a
saved passenger sets only its private reference to null; the hold's name snapshot
survives. Existing history prevents physical trip deletion by a foreign key.
Changing/cancelling a trip requires cancelling/expiring its active holds first;
there is no automatic payment or notification workflow.

For `held` rows, all TTL comparisons use PostgreSQL clocks. `payment_committing`
and `confirmed` are protected independently of TTL; availability counts both and
cleanup never expires them. Expiry is at most 600 seconds and is
shortened to departure when it is sooner. Reading a stale `held` row returns its
effective `expired` status. Availability excludes expired rows even without the
worker. Create expires stale exclusion entries for its trip before allocation,
using the same captured DB timestamp for cleanup and conflict checks. Cleanup is
an optimization, not a stock prerequisite.

Idempotency keys are mandatory, 8–128 ASCII alphanumeric/underscore/hyphen, unique
per owner. The hash covers canonical validated segment/passenger input. A retry
returns the original snapshot and expiry (HTTP200), including cancelled/expired
holds and deleted saved-passenger templates. A different payload with that key
returns409. New holds return201. Failed transactions do not consume a key.

`Expire(ctx, limit)` processes at most1000 (root scheduler uses250) due rows via
`FOR UPDATE SKIP LOCKED`. Multiple instances may run it safely. Five-second
contexts cancel database work; the root application owns the cancellable scheduler.
No process-wide worker starts during package import or tests.

## HTTP API

Machine-readable additive contract: [booking.openapi.json](booking.openapi.json)
(OpenAPI3.0.3; booking/passengers and catalogue quality). Existing route search
keeps [routes.openapi.json](routes.openapi.json).

Base `/api/v1`:

- Public `GET /trips/:id/availability?boarding_stop_id=…&alighting_stop_id=…`.
- Authenticated `POST /me/bookings` with `Idempotency-Key` and exactly one of
  `passenger_id` or `passenger_name`, plus trip and concrete boarding/alighting IDs.
- Authenticated `GET /me/bookings`, `GET /me/bookings/:id`,
  `POST /me/bookings/:id/cancel` (idempotent).
- Authenticated `GET /me/passengers`, `POST /me/passengers`,
  `PUT /me/passengers/:id`, `DELETE /me/passengers/:id`.

Lists use ID keysets, default30/max100, owner-and-resource-scoped cursors, limit+1,
no OFFSET, no total COUNT. History is one SQL regardless of page size: all display
values come from the hold snapshot, with no per-trip/passenger follow-up queries.
Names are trimmed, 1–160 Unicode characters, with controls rejected. Request
bodies are bounded4096 bytes, reject unknown fields and trailing JSON values.
IDs are positive; integer trip/stop IDs are checked for PostgreSQL INTEGER bounds.
Foreign/missing records return404 without leaking ownership. Errors include a
stable code, with private SQL/driver errors hidden. Clients should preserve their
idempotency key across network retries and clear it for a genuinely new attempt.

Root wiring: `handler.NewBooking(db).Register(public, authorized)` and a cancellable
scheduler calling `Expire(ctx,250)`; authorization belongs on `authorized` before
registration. These APIs do not activate legacy checkout.

## Verification

Run against an explicitly provided disposable `TEST_DATABASE_URL`, never the dev
application database. Fixtures create/drop random schemas and use synthetic
accounts. Go race tests cover simultaneous last-seat allocation, adjacent and
overlapping intervals, direct DB exclusion, cross-trip owner limit, concurrent
idempotent retries and payload conflict, owner/template isolation, deletion/name
and fare snapshots, expiration without cleanup and bounded cleanup, concurrent
fare/status writes, guarded trip/bus changes and smaller-capacity allocation,
repeated-stop ambiguity, read-only real/demo availability,
cursor scoping, 1/30/100-row constant query counts and migration down/up. Handler
tests cover strict inputs, auth identity, overflow and safe error responses.

Retention, real-carrier inventory synchronization, live payment, real ticket
issuance, user partial refunds, notification delivery and document storage remain
outside this MVP. Sandbox authorization/capture/compensation are documented in
[CHECKOUT_PROTOCOL.md](CHECKOUT_PROTOCOL.md).
Hold history currently has no automatic retention policy.
