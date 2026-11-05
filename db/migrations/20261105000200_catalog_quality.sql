-- +goose Up
-- +goose StatementBegin
ALTER TABLE stops ADD COLUMN quality_version bigint NOT NULL DEFAULT 1;
CREATE INDEX stops_quality_queue ON stops(id) WHERE city_id IS NULL;
CREATE TABLE catalog_mapping_audit (
 id bigserial PRIMARY KEY,
 stop_id integer NOT NULL REFERENCES stops(id) ON DELETE CASCADE,
 actor_id integer NOT NULL REFERENCES users(id),
 reason text NOT NULL CHECK(char_length(btrim(reason)) BETWEEN 5 AND 500),
 before_mapping jsonb NOT NULL,
 after_mapping jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX catalog_mapping_audit_stop ON catalog_mapping_audit(stop_id,id DESC);

-- The UPDATE row lock conflicts with the SHARE lock taken before inserting a
-- schedule reference. Quality mapping checks references after taking that lock.
CREATE FUNCTION catalog_quality_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.quality_version := OLD.quality_version+1;
 RETURN NEW;
END $$;
CREATE TRIGGER catalog_quality_guard BEFORE UPDATE ON stops FOR EACH ROW EXECUTE FUNCTION catalog_quality_guard();
CREATE FUNCTION catalog_quality_schedule_lock() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM stops WHERE id=NEW.stop_id FOR SHARE;
 RETURN NEW;
END $$;
CREATE TRIGGER catalog_quality_schedule_lock BEFORE INSERT OR UPDATE OF stop_id ON trip_stops FOR EACH ROW EXECUTE FUNCTION catalog_quality_schedule_lock();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER catalog_quality_schedule_lock ON trip_stops;
DROP TRIGGER catalog_quality_guard ON stops;
DROP FUNCTION catalog_quality_schedule_lock(),catalog_quality_guard();
DROP TABLE catalog_mapping_audit;
DROP INDEX stops_quality_queue;
ALTER TABLE stops DROP COLUMN quality_version;
-- +goose StatementEnd
