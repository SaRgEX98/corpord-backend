# ADR 0001: route money, time, and informational availability

Date: 2026-11-01. Scope: first informational search, not sales or demand pricing.

Existing TIMESTAMP columns are treated as UTC instants, matching legacy API
serialization with Z. New search explicitly interprets UTC in PostgreSQL and
converts departure/arrival independently to confirmed IANA zones. No mass timestamp
rewrite or geographical timezone inference is safe. Stop city assignment is
explicit and existing unmapped stops remain accessible. Before deployment operators
must audit real imported timestamp semantics; data that was stored as local wall
clock requires a separate reviewed correction.

Money is RUB in integer kopecks in new APIs. Existing NUMERIC(10,2) storage is exact;
generated BIGINT columns multiply it by 100 in PostgreSQL, without floating point.
Legacy float JSON writes remain supported; database scale/rounding semantics are
unchanged. Generated values cannot diverge on old-client edits. New writes of
base/leg tariffs still use the legacy admin endpoints; a future sales write API
must accept minor units directly. Migration rollback drops only generated columns
and location metadata; existing decimal values and schedules remain intact. City
assignments/overrides must be exported before rollback if retention is required.

The informational section tariff is the sum of price_to_next for all legs
[boarding, alighting). The independent legacy base_price is NOT silently added,
prorated or used as a fallback. This avoids double charging and invented partial
prices. Zero is a valid free leg; NULL/negative means tariff unconfirmed, so price
is null. This is an explicit MVP interpretation of the existing column, not a
carrier-approved sales fare. Product/carrier approval of tariff policy and fees
remains a prerequisite to selling tickets. There are no automatic multipliers,
seasonality, user-specific prices, holds or quoted-price guarantee in this phase.

All APIs use the same section SQL and rule version segment-v1. Quote recomputes
now, returns no expiry, and never reserves a seat. Seats remain unknown even if
bus capacity or historical order rows exist. Demand pricing remains disabled by
absence of any dynamic pricing implementation; only verified inventory and a
separately approved audited tariff policy can activate a later phase.
