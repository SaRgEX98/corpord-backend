-- +goose Up
-- +goose StatementBegin
-- Stop the old API/workers before upgrade: all seat writers must use this schema.
ALTER TABLE booking_holds DROP CONSTRAINT booking_holds_status_check;
ALTER TABLE booking_holds ADD CONSTRAINT booking_holds_status_check CHECK(status IN ('held','cancelled','expired','payment_committing','confirmed'));
DO $$ DECLARE n text; BEGIN
 SELECT conname INTO n FROM pg_constraint WHERE conrelid='booking_holds'::regclass AND contype='x';
 EXECUTE format('ALTER TABLE booking_holds DROP CONSTRAINT %I',n);
END $$;
ALTER TABLE booking_holds ADD CONSTRAINT booking_holds_seat_exclusion EXCLUDE USING gist (trip_id WITH =,seat_number WITH =,occupied_range WITH &&) WHERE(status IN ('held','payment_committing','confirmed'));
CREATE INDEX booking_holds_protected_trip ON booking_holds(trip_id) WHERE status IN ('payment_committing','confirmed');
CREATE TABLE checkout_orders (
 id BIGSERIAL PRIMARY KEY,
 user_id integer NOT NULL REFERENCES users(id),
 hold_id bigint NOT NULL UNIQUE REFERENCES booking_holds(id),
 trip_id integer NOT NULL REFERENCES trips(id),
 status text NOT NULL DEFAULT 'awaiting_payment' CHECK(status IN ('awaiting_payment','committing','paid','cancel_pending','cancelled','expired','review_required','refund_pending','refunded')),
 payment_status text NOT NULL DEFAULT 'not_started' CHECK(payment_status IN ('not_started','pending','authorized','processing','succeeded','canceled','unknown','refunding','refunded')),
 amount_minor bigint NOT NULL CHECK(amount_minor BETWEEN 100 AND 35000000),
 currency text NOT NULL CHECK(currency='RUB'),
 is_demo boolean NOT NULL CHECK(is_demo),
 test boolean NOT NULL DEFAULT true CHECK(test),
 passenger_name text NOT NULL,
 boarding_name text NOT NULL,
 alighting_name text NOT NULL,
 departure_time timestamptz NOT NULL,
 arrival_time timestamptz NOT NULL,
 timezone text NOT NULL,
 arrival_timezone text NOT NULL,
 seat_number integer NOT NULL,
 hold_expires_at timestamptz NOT NULL,
 idempotency_key text NOT NULL,
 cancel_requested boolean NOT NULL DEFAULT false,
 last_reconcile_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(user_id,idempotency_key)
);
CREATE INDEX checkout_orders_owner_page ON checkout_orders(user_id,id DESC);
CREATE INDEX checkout_orders_owner_status_page ON checkout_orders(user_id,status,id DESC);
CREATE INDEX checkout_orders_recovery ON checkout_orders(updated_at,id) WHERE status IN ('awaiting_payment','committing','cancel_pending','refund_pending');
ALTER TABLE booking_holds ADD COLUMN checkout_id bigint UNIQUE REFERENCES checkout_orders(id);
CREATE TABLE checkout_attempts (
 order_id bigint PRIMARY KEY REFERENCES checkout_orders(id),
 intent_id text NOT NULL UNIQUE,
 provider_id text UNIQUE,
 merchant_id text,
 provider_status text,
 confirmation_url text,
 create_key uuid NOT NULL UNIQUE,
 capture_key uuid NOT NULL UNIQUE,
 cancel_key uuid NOT NULL UNIQUE,
 refund_key uuid NOT NULL UNIQUE,
 refund_id text UNIQUE,
 refund_status text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE checkout_jobs (
 id BIGSERIAL PRIMARY KEY,
 order_id bigint NOT NULL REFERENCES checkout_orders(id),
 kind text NOT NULL CHECK(kind IN ('create','reconcile','capture','cancel','refund','reconcile_refund')),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','leased','done')),
 attempts integer NOT NULL DEFAULT 0,
 failures integer NOT NULL DEFAULT 0,
 network_started boolean NOT NULL DEFAULT false,
 available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 lease_until timestamptz,
 fence bigint NOT NULL DEFAULT 0,
 last_error text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(order_id,kind)
);
CREATE INDEX checkout_jobs_ready ON checkout_jobs(available_at,id) WHERE state='pending';
CREATE INDEX checkout_jobs_reclaim ON checkout_jobs(lease_until,id) WHERE state='leased';
CREATE TABLE checkout_inbox (
 id BIGSERIAL PRIMARY KEY,
 event text NOT NULL,
 object_id text NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','leased','done')),
 attempts integer NOT NULL DEFAULT 0,
 failures integer NOT NULL DEFAULT 0,
 available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 lease_until timestamptz,
 fence bigint NOT NULL DEFAULT 0,
 last_error text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(event,object_id)
);
CREATE INDEX checkout_inbox_ready ON checkout_inbox(available_at,id) WHERE state='pending';
CREATE INDEX checkout_inbox_reclaim ON checkout_inbox(lease_until,id) WHERE state='leased';
CREATE OR REPLACE FUNCTION booking_guard_timetable() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE affected INTEGER; now_db TIMESTAMPTZ := clock_timestamp();
BEGIN
 FOR affected IN SELECT DISTINCT n FROM unnest(CASE WHEN TG_OP='INSERT' THEN ARRAY[NEW.trip_id] WHEN TG_OP='DELETE' THEN ARRAY[OLD.trip_id] ELSE ARRAY[OLD.trip_id,NEW.trip_id] END) n ORDER BY n LOOP
  PERFORM id FROM trips WHERE id=affected FOR UPDATE;
  IF EXISTS (SELECT 1 FROM booking_holds WHERE trip_id=affected AND (status IN ('payment_committing','confirmed') OR (status='held' AND expires_at>now_db))) THEN
   RAISE EXCEPTION USING ERRCODE='23514', MESSAGE='booking_trip_locked';
  END IF;
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION booking_guard_trip() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' AND ROW(NEW.bus_id,NEW.driver_id,NEW.start_time,NEW.end_time,NEW.status,NEW.base_price,NEW.is_demo) IS NOT DISTINCT FROM ROW(OLD.bus_id,OLD.driver_id,OLD.start_time,OLD.end_time,OLD.status,OLD.base_price,OLD.is_demo) THEN RETURN NEW; END IF;
 IF EXISTS (SELECT 1 FROM booking_holds WHERE trip_id=OLD.id AND (status IN ('payment_committing','confirmed') OR (status='held' AND expires_at>clock_timestamp()))) THEN
  RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='booking_trip_locked';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
-- A hold reads the bus under SHARE; this row-level writer lock therefore
-- serializes a capacity decrease with allocation without reversing trip locks.
CREATE OR REPLACE FUNCTION booking_guard_bus() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' AND NEW.capacity>=OLD.capacity AND NEW.deleted_at IS NOT DISTINCT FROM OLD.deleted_at THEN RETURN NEW; END IF;
 IF EXISTS (SELECT 1 FROM trips t JOIN booking_holds h ON h.trip_id=t.id WHERE t.bus_id=OLD.id AND (h.status='payment_committing' OR (h.status='confirmed' AND h.arrival_time>clock_timestamp()) OR (h.status='held' AND h.expires_at>clock_timestamp()))) THEN
  RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='booking_bus_locked';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM checkout_orders) OR EXISTS(SELECT 1 FROM checkout_inbox) OR EXISTS(SELECT 1 FROM booking_holds WHERE status IN ('payment_committing','confirmed')) THEN
  RAISE EXCEPTION 'checkout rollback refused: payment/seat history must be reconciled and archived explicitly';
 END IF;
END $$;
ALTER TABLE booking_holds DROP COLUMN checkout_id;
DROP TABLE checkout_inbox,checkout_jobs,checkout_attempts,checkout_orders;
DROP INDEX booking_holds_protected_trip;
ALTER TABLE booking_holds DROP CONSTRAINT booking_holds_seat_exclusion;
ALTER TABLE booking_holds ADD CONSTRAINT booking_holds_seat_exclusion EXCLUDE USING gist(trip_id WITH =,seat_number WITH =,occupied_range WITH &&) WHERE(status='held');
ALTER TABLE booking_holds DROP CONSTRAINT booking_holds_status_check;
ALTER TABLE booking_holds ADD CONSTRAINT booking_holds_status_check CHECK(status IN ('held','cancelled','expired'));
CREATE OR REPLACE FUNCTION booking_guard_timetable() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE affected INTEGER; now_db TIMESTAMPTZ := clock_timestamp();
BEGIN
 FOR affected IN SELECT DISTINCT n FROM unnest(CASE WHEN TG_OP='INSERT' THEN ARRAY[NEW.trip_id] WHEN TG_OP='DELETE' THEN ARRAY[OLD.trip_id] ELSE ARRAY[OLD.trip_id,NEW.trip_id] END) n ORDER BY n LOOP
  PERFORM id FROM trips WHERE id=affected FOR UPDATE;
  IF EXISTS (SELECT 1 FROM booking_holds WHERE trip_id=affected AND status='held' AND expires_at>now_db) THEN
   RAISE EXCEPTION USING ERRCODE='23514', MESSAGE='booking_trip_locked';
  END IF;
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION booking_guard_trip() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' AND ROW(NEW.bus_id,NEW.driver_id,NEW.start_time,NEW.end_time,NEW.status,NEW.base_price,NEW.is_demo) IS NOT DISTINCT FROM ROW(OLD.bus_id,OLD.driver_id,OLD.start_time,OLD.end_time,OLD.status,OLD.base_price,OLD.is_demo) THEN RETURN NEW; END IF;
 IF EXISTS (SELECT 1 FROM booking_holds WHERE trip_id=OLD.id AND status='held' AND expires_at>clock_timestamp()) THEN
  RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='booking_trip_locked';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
-- A hold reads the bus under SHARE; this row-level writer lock therefore
-- serializes a capacity decrease with allocation without reversing trip locks.
CREATE OR REPLACE FUNCTION booking_guard_bus() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' AND NEW.capacity>=OLD.capacity AND NEW.deleted_at IS NOT DISTINCT FROM OLD.deleted_at THEN RETURN NEW; END IF;
 IF EXISTS (SELECT 1 FROM trips t JOIN booking_holds h ON h.trip_id=t.id WHERE t.bus_id=OLD.id AND h.status='held' AND h.expires_at>clock_timestamp()) THEN
  RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='booking_bus_locked';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
