-- +goose Up
-- +goose StatementBegin
alter table trips
drop
constraint trips_bus_id_fkey;

alter table bus
drop
constraint bus_pkey;

alter table bus
    add id serial
        primary key;

alter table bus
    add constraint bus_pk
        unique (license_plate);

alter table trips
alter
column bus_id type integer using bus_id::integer;

alter table trips
    add constraint trips_bus_id_fkey
        foreign key (bus_id) references bus;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table trips
drop
constraint trips_bus_id_fkey;

alter table bus
drop
constraint bus_pkey;

alter table bus
drop
constraint bus_pk;

alter table bus
    add primary key (license_plate);

alter table bus
drop
id;

alter table trips
alter
column bus_id type text using bus_id::text;

alter table trips
    add constraint trips_bus_id_fkey
        foreign key (bus_id) references bus;
-- +goose StatementEnd
