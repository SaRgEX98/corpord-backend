-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

CREATE FUNCTION catalog_normalize(value text) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
 SELECT regexp_replace(translate(lower(btrim(coalesce(value,''))),
 'АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯё',
 'абвгдеежзийклмнопрстуфхцчшщъыьэюяе'), '\s+', ' ', 'g')
$$;

ALTER TABLE route_cities ALTER COLUMN timezone DROP NOT NULL;
ALTER TABLE route_cities DROP CONSTRAINT route_cities_name_region_key;
CREATE INDEX route_cities_name_region ON route_cities(name,region);
CREATE INDEX route_cities_region ON route_cities(region,id);
CREATE TABLE catalog_source_links(
 source_key text PRIMARY KEY,kind text NOT NULL CHECK(kind IN ('city','stop')),
 entity_id integer NOT NULL,owned boolean NOT NULL, UNIQUE(kind,entity_id,source_key)
);
ALTER TABLE route_cities ADD COLUMN latitude double precision, ADD COLUMN longitude double precision,
 ADD COLUMN source_key text UNIQUE, ADD COLUMN source_snapshot jsonb;
ALTER TABLE stops ADD COLUMN source_key text UNIQUE, ADD COLUMN source_snapshot jsonb,
 ADD COLUMN stop_kind text NOT NULL DEFAULT 'bus_stop';
ALTER TABLE trips ADD COLUMN is_demo boolean NOT NULL DEFAULT false;

CREATE TABLE route_regions(id serial PRIMARY KEY, name text NOT NULL UNIQUE);
CREATE TABLE catalog_search(
 kind text NOT NULL CHECK(kind IN ('city','stop','region')), entity_id integer NOT NULL,
 name text NOT NULL, region text NOT NULL DEFAULT '', city_id integer,
 address text NOT NULL DEFAULT '', city_name text NOT NULL DEFAULT '', timezone text, latitude double precision, longitude double precision,
 popularity integer NOT NULL DEFAULT 0,
 name_norm text COLLATE "C" GENERATED ALWAYS AS (catalog_normalize(name)) STORED,
 document text GENERATED ALWAYS AS (catalog_normalize(name || ' ' || city_name || ' ' || region || ' ' || address)) STORED,
 PRIMARY KEY(kind,entity_id)
);
CREATE INDEX catalog_prefix ON catalog_search(kind,name_norm,entity_id);
CREATE INDEX catalog_city_prefix ON catalog_search(city_id,kind,name_norm,entity_id);
CREATE INDEX catalog_region_prefix ON catalog_search(region,kind,name_norm,entity_id);
CREATE INDEX catalog_popular ON catalog_search(kind,popularity DESC,entity_id);
CREATE INDEX catalog_name_fuzzy ON catalog_search USING gist(name_norm public.gist_trgm_ops(siglen=64));
CREATE INDEX catalog_document ON catalog_search USING gin(document public.gin_trgm_ops);
-- Departure bounds and stop membership are tested before route/fare expansion.
CREATE INDEX stops_city_page ON stops(city_id,id);
CREATE INDEX trip_stops_stop_departure ON trip_stops(stop_id,departure_time,trip_id) INCLUDE(stop_order);

CREATE FUNCTION catalog_sync_region() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO catalog_search(kind,entity_id,name,region) VALUES('region',NEW.id,NEW.name,NEW.name)
 ON CONFLICT(kind,entity_id) DO UPDATE SET name=EXCLUDED.name,region=EXCLUDED.region;
 RETURN NEW;
END $$;
CREATE TRIGGER catalog_region AFTER INSERT OR UPDATE ON route_regions FOR EACH ROW EXECUTE FUNCTION catalog_sync_region();

CREATE FUNCTION catalog_sync_city() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  DELETE FROM catalog_search WHERE kind='city' AND entity_id=OLD.id;
  RETURN OLD;
 END IF;
 IF TG_OP='UPDATE' AND (NEW.name,NEW.region,NEW.timezone,NEW.popularity,NEW.latitude,NEW.longitude) IS NOT DISTINCT FROM (OLD.name,OLD.region,OLD.timezone,OLD.popularity,OLD.latitude,OLD.longitude) THEN RETURN NEW; END IF;
 INSERT INTO route_regions(name) VALUES(NEW.region) ON CONFLICT(name) DO NOTHING;
 INSERT INTO catalog_search(kind,entity_id,name,region,city_id,timezone,popularity,latitude,longitude)
 VALUES('city',NEW.id,NEW.name,NEW.region,NEW.id,NEW.timezone,NEW.popularity,NEW.latitude,NEW.longitude)
 ON CONFLICT(kind,entity_id) DO UPDATE SET name=EXCLUDED.name,region=EXCLUDED.region,
 timezone=EXCLUDED.timezone,popularity=EXCLUDED.popularity,latitude=EXCLUDED.latitude,longitude=EXCLUDED.longitude;
 IF TG_OP='UPDATE' THEN
  UPDATE catalog_search cs SET city_name=NEW.name,region=NEW.region,timezone=coalesce(s.timezone,NEW.timezone)
  FROM stops s WHERE cs.kind='stop' AND cs.entity_id=s.id AND s.city_id=NEW.id;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER catalog_city AFTER INSERT OR UPDATE OR DELETE ON route_cities FOR EACH ROW EXECUTE FUNCTION catalog_sync_city();

CREATE FUNCTION catalog_sync_stop() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  DELETE FROM catalog_search WHERE kind='stop' AND entity_id=OLD.id;
  RETURN OLD;
 END IF;
 IF TG_OP='UPDATE' AND (NEW.name,NEW.address,NEW.city_id,NEW.timezone,NEW.latitude,NEW.longitude) IS NOT DISTINCT FROM (OLD.name,OLD.address,OLD.city_id,OLD.timezone,OLD.latitude,OLD.longitude) THEN RETURN NEW; END IF;
 INSERT INTO catalog_search(kind,entity_id,name,region,city_id,address,city_name,timezone,latitude,longitude)
 SELECT 'stop',NEW.id,NEW.name,coalesce(c.region,''),NEW.city_id,coalesce(NEW.address,''),coalesce(c.name,''),
 coalesce(NEW.timezone,c.timezone),NEW.latitude::float8,NEW.longitude::float8
 FROM (SELECT 1) seed LEFT JOIN route_cities c ON c.id=NEW.city_id
 ON CONFLICT(kind,entity_id) DO UPDATE SET name=EXCLUDED.name,region=EXCLUDED.region,city_id=EXCLUDED.city_id,
 address=EXCLUDED.address,city_name=EXCLUDED.city_name,timezone=EXCLUDED.timezone,latitude=EXCLUDED.latitude,longitude=EXCLUDED.longitude;
 RETURN NEW;
END $$;
CREATE TRIGGER catalog_stop AFTER INSERT OR UPDATE OR DELETE ON stops FOR EACH ROW EXECUTE FUNCTION catalog_sync_stop();
-- Backfill without rewriting existing domain rows.
INSERT INTO route_regions(name) SELECT DISTINCT region FROM route_cities;
INSERT INTO catalog_search(kind,entity_id,name,region,city_id,timezone,popularity,latitude,longitude)
 SELECT 'city',id,name,region,id,timezone,popularity,latitude,longitude FROM route_cities;
INSERT INTO catalog_search(kind,entity_id,name,region,city_id,address,city_name,timezone,latitude,longitude)
 SELECT 'stop',s.id,s.name,coalesce(c.region,''),s.city_id,coalesce(s.address,''),coalesce(c.name,''),coalesce(s.timezone,c.timezone),s.latitude::float8,s.longitude::float8
 FROM stops s LEFT JOIN route_cities c ON c.id=s.city_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER catalog_stop ON stops;
DROP TRIGGER catalog_city ON route_cities;
DROP TRIGGER catalog_region ON route_regions;
DROP FUNCTION catalog_sync_stop(),catalog_sync_city(),catalog_sync_region();
DROP INDEX trip_stops_stop_departure,stops_city_page;
DROP TABLE catalog_search,route_regions,catalog_source_links;
DROP INDEX route_cities_name_region,route_cities_region;
ALTER TABLE route_cities ADD UNIQUE(name,region);
DROP FUNCTION catalog_normalize(text);
ALTER TABLE trips DROP COLUMN is_demo;
ALTER TABLE stops DROP COLUMN source_key,DROP COLUMN source_snapshot,DROP COLUMN stop_kind;
ALTER TABLE route_cities DROP COLUMN latitude,DROP COLUMN longitude,DROP COLUMN source_key,DROP COLUMN source_snapshot;
-- Unknown imported timezones cannot be silently converted to UTC on rollback.
ALTER TABLE route_cities ALTER COLUMN timezone SET NOT NULL;
-- pg_trgm is database-scoped and may be shared by other schemas.
-- +goose StatementEnd
