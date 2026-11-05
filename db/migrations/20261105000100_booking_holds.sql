-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;
CREATE TABLE booking_passengers (
 id BIGSERIAL PRIMARY KEY,
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 full_name TEXT NOT NULL CHECK (full_name=btrim(full_name) AND char_length(full_name) BETWEEN 1 AND 160),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX booking_passengers_owner_page ON booking_passengers(user_id,id DESC);
-- Created on the first hold, never by availability reads or bulk generation.
CREATE TABLE booking_inventory (
 trip_id INTEGER PRIMARY KEY REFERENCES trips(id) ON DELETE CASCADE,
 capacity INTEGER NOT NULL CHECK (capacity BETWEEN 1 AND 1000),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE booking_holds (
 id BIGSERIAL PRIMARY KEY,
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 trip_id INTEGER NOT NULL REFERENCES trips(id),
 passenger_id BIGINT REFERENCES booking_passengers(id) ON DELETE SET NULL,
 passenger_name TEXT NOT NULL,
 boarding_stop_id INTEGER NOT NULL,
 alighting_stop_id INTEGER NOT NULL,
 boarding_order INTEGER NOT NULL CHECK (boarding_order>0),
 alighting_order INTEGER NOT NULL CHECK (alighting_order>boarding_order),
 boarding_name TEXT NOT NULL,
 alighting_name TEXT NOT NULL,
 departure_time TIMESTAMPTZ NOT NULL,
 arrival_time TIMESTAMPTZ NOT NULL CHECK (arrival_time>departure_time),
 timezone TEXT NOT NULL,
 arrival_timezone TEXT NOT NULL,
 seat_number INTEGER NOT NULL CHECK (seat_number BETWEEN 1 AND 1000),
 amount_minor BIGINT NOT NULL CHECK (amount_minor>=0),
 currency TEXT NOT NULL DEFAULT 'RUB' CHECK (currency='RUB'),
 is_demo BOOLEAN NOT NULL CHECK (is_demo),
 status TEXT NOT NULL DEFAULT 'held' CHECK (status IN ('held','cancelled','expired')),
 created_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at>created_at),
 cancelled_at TIMESTAMPTZ,
 idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 8 AND 128),
 payload_hash TEXT NOT NULL,
 occupied_range INT4RANGE GENERATED ALWAYS AS (int4range(boarding_order,alighting_order,'[)')) STORED,
 UNIQUE(user_id,idempotency_key),
 EXCLUDE USING gist (trip_id WITH =,seat_number WITH =,occupied_range WITH &&) WHERE (status='held')
);
CREATE INDEX booking_holds_owner_page ON booking_holds(user_id,id DESC);
CREATE INDEX booking_holds_owner_active ON booking_holds(user_id,expires_at) WHERE status='held';
CREATE INDEX booking_holds_trip_active ON booking_holds(trip_id,expires_at) WHERE status='held';
CREATE INDEX booking_holds_expiry ON booking_holds(expires_at,id) WHERE status='held';
-- All timetable writes acquire the parent lock also used by hold creation. A
-- held seat's interval cannot be reinterpreted by concurrent stop reordering.
CREATE FUNCTION booking_guard_timetable() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER booking_guard_timetable BEFORE INSERT OR UPDATE OR DELETE ON trip_stops FOR EACH ROW EXECUTE FUNCTION booking_guard_timetable();
CREATE FUNCTION booking_guard_trip() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' AND ROW(NEW.bus_id,NEW.driver_id,NEW.start_time,NEW.end_time,NEW.status,NEW.base_price,NEW.is_demo) IS NOT DISTINCT FROM ROW(OLD.bus_id,OLD.driver_id,OLD.start_time,OLD.end_time,OLD.status,OLD.base_price,OLD.is_demo) THEN RETURN NEW; END IF;
 IF EXISTS (SELECT 1 FROM booking_holds WHERE trip_id=OLD.id AND status='held' AND expires_at>clock_timestamp()) THEN
  RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='booking_trip_locked';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER booking_guard_trip BEFORE UPDATE OR DELETE ON trips FOR EACH ROW EXECUTE FUNCTION booking_guard_trip();
-- A hold reads the bus under SHARE; this row-level writer lock therefore
-- serializes a capacity decrease with allocation without reversing trip locks.
CREATE FUNCTION booking_guard_bus() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' AND NEW.capacity>=OLD.capacity AND NEW.deleted_at IS NOT DISTINCT FROM OLD.deleted_at THEN RETURN NEW; END IF;
 IF EXISTS (SELECT 1 FROM trips t JOIN booking_holds h ON h.trip_id=t.id WHERE t.bus_id=OLD.id AND h.status='held' AND h.expires_at>clock_timestamp()) THEN
  RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='booking_bus_locked';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER booking_guard_bus BEFORE UPDATE OF capacity,deleted_at OR DELETE ON bus FOR EACH ROW EXECUTE FUNCTION booking_guard_bus();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER booking_guard_bus ON bus;
DROP FUNCTION booking_guard_bus();
DROP TRIGGER booking_guard_trip ON trips;
DROP FUNCTION booking_guard_trip();
DROP TRIGGER booking_guard_timetable ON trip_stops;
DROP FUNCTION booking_guard_timetable();
DROP TABLE booking_holds;
DROP TABLE booking_inventory;
DROP TABLE booking_passengers;
-- btree_gist may be used by other modules; retain the shared extension.
-- +goose StatementEnd
