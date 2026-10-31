-- +goose Up
-- +goose StatementBegin
CREATE TABLE route_cities (
 id SERIAL PRIMARY KEY,
 name TEXT NOT NULL CHECK (btrim(name) <> ''),
 region TEXT NOT NULL CHECK (btrim(region) <> ''),
 timezone TEXT NOT NULL,
 popularity INT NOT NULL DEFAULT 0 CHECK (popularity >= 0),
 UNIQUE(name,region)
);
INSERT INTO route_cities(name,region,timezone,popularity) VALUES
 ('Москва','Москва','Europe/Moscow',100),
 ('Екатеринбург','Свердловская область','Asia/Yekaterinburg',80),
 ('Нижний Тагил','Свердловская область','Asia/Yekaterinburg',40),
 ('Челябинск','Челябинская область','Asia/Yekaterinburg',50),
 ('Тюмень','Тюменская область','Asia/Yekaterinburg',50),
 ('Пермь','Пермский край','Asia/Yekaterinburg',50);
ALTER TABLE stops ADD COLUMN city_id INT REFERENCES route_cities(id), ADD COLUMN timezone TEXT;
CREATE INDEX idx_stops_city_id ON stops(city_id);
-- UTC timestamps and legacy decimal writes are retained. Generated columns cannot drift.
ALTER TABLE trips ADD COLUMN base_price_minor BIGINT GENERATED ALWAYS AS ((base_price * 100)::BIGINT) STORED;
ALTER TABLE trip_stops ADD COLUMN price_to_next_minor BIGINT GENERATED ALWAYS AS ((price_to_next * 100)::BIGINT) STORED;
CREATE INDEX idx_route_segments ON trip_stops(stop_id,trip_id,stop_order);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX idx_route_segments;
ALTER TABLE trip_stops DROP COLUMN price_to_next_minor;
ALTER TABLE trips DROP COLUMN base_price_minor;
ALTER TABLE stops DROP COLUMN city_id, DROP COLUMN timezone;
DROP TABLE route_cities;
-- +goose StatementEnd
