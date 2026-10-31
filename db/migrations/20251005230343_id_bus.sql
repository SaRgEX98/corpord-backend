-- This migration repairs the upgrade path for installations that have not applied
-- version 20251005230343 yet. Goose does not re-run it on an upgraded database.
-- +goose Up
-- +goose StatementBegin
ALTER TABLE trips DROP CONSTRAINT trips_bus_id_fkey;

ALTER TABLE bus ADD COLUMN id SERIAL;
ALTER TABLE trips ADD COLUMN bus_id_new INTEGER;

-- The legacy foreign key stores a license plate, not the new numeric bus ID.
UPDATE trips AS t
SET bus_id_new = b.id
FROM bus AS b
WHERE t.bus_id = b.license_plate;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM trips WHERE bus_id_new IS NULL) THEN
        RAISE EXCEPTION 'Cannot migrate trips: a legacy bus reference has no matching license plate';
    END IF;
END $$;

ALTER TABLE trips ALTER COLUMN bus_id_new SET NOT NULL;
ALTER TABLE trips DROP COLUMN bus_id;
ALTER TABLE trips RENAME COLUMN bus_id_new TO bus_id;
CREATE INDEX idx_trips_bus_id ON trips(bus_id);

ALTER TABLE bus DROP CONSTRAINT bus_pkey;
ALTER TABLE bus ADD CONSTRAINT bus_pkey PRIMARY KEY (id);
ALTER TABLE bus ADD CONSTRAINT bus_pk UNIQUE (license_plate);
ALTER TABLE trips ADD CONSTRAINT trips_bus_id_fkey FOREIGN KEY (bus_id) REFERENCES bus(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE trips ADD COLUMN bus_plate_old TEXT;
UPDATE trips AS t
SET bus_plate_old = b.license_plate
FROM bus AS b
WHERE t.bus_id = b.id;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM trips WHERE bus_plate_old IS NULL) THEN
        RAISE EXCEPTION 'Cannot roll back trips: a bus ID has no matching license plate';
    END IF;
END $$;

ALTER TABLE trips ALTER COLUMN bus_plate_old SET NOT NULL;
ALTER TABLE trips DROP CONSTRAINT trips_bus_id_fkey;
ALTER TABLE trips DROP COLUMN bus_id;
ALTER TABLE trips RENAME COLUMN bus_plate_old TO bus_id;
CREATE INDEX idx_trips_bus_id ON trips(bus_id);

ALTER TABLE bus DROP CONSTRAINT bus_pkey;
ALTER TABLE bus DROP CONSTRAINT bus_pk;
ALTER TABLE bus ADD CONSTRAINT bus_pkey PRIMARY KEY (license_plate);
ALTER TABLE trips ADD CONSTRAINT trips_bus_id_fkey FOREIGN KEY (bus_id) REFERENCES bus(license_plate);
ALTER TABLE bus DROP COLUMN id;
-- +goose StatementEnd
