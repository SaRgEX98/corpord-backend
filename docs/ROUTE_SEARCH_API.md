# Informational route API v1

Base `/api/v1`. Legacy endpoints remain. Search is public, city changes require admin.

- `GET /locations/suggest?q=&origin_id=city:1&limit=8` → `{items: Location[]}`.
- `GET /locations/{city:N|stop:N}` → canonical Location object (for URL restoration).
- `GET /locations/{city:N|stop:N}/stops` → `{items: Location[]}`.
- `GET /trips/search?from_id=stop:70&to_id=city:2&date=2026-12-01&passengers=1&sort=departure&limit=20&cursor=...` → `{items: Segment[],next_cursor: string|null}`.
- `GET /trips/calendar?from_id=...&to_id=...&month=2026-12&passengers=1` → calendar below.
- `POST /trips/{id}/quote` body `{boarding_stop_id:70,alighting_stop_id:71,passengers:1}` → Segment plus `{expires_at:null,reserves_seat:false}`.

Location: `{id:string,type:'city'|'stop',name:string,region:string,timezone:string|null,served:boolean,rank_reason:'match'|'connected'|'popular',address:string,latitude:number|null,longitude:number|null}`.
Cities have null coordinates. IDs are stable internal IDs, never Photon IDs.
Empty query returns only served locations (future scheduled/boarding/delayed trip),
ranked by connectivity, then served status and configured initial priority.
Nonempty query also finds unserved locations, marked `served:false`.
Name exact/prefix/substring matching takes precedence over connectivity/popularity;
Russian ё/е are normalized, `%` and `_` are literal. Query maximum 120 characters,
limit 1–20. Origin direction means boarding occurs before destination.

Segment:
```json
{
  "trip_id":80,"boarding_stop_id":70,"alighting_stop_id":71,
  "boarding_name":"Start","alighting_name":"End",
  "departure_time":"2026-12-01T13:10:00+05:00",
  "arrival_time":"2026-12-01T16:30:00+05:00",
  "timezone":"Asia/Yekaterinburg","arrival_timezone":"Asia/Yekaterinburg",
  "status":"scheduled",
  "price":{"amount_minor":12345,"currency":"RUB","rule_version":"segment-v1","calculated_at":"2026-11-01T10:00:00Z"},
  "availability":{"status":"unknown","seats":null}
}
```
`price` is null if any leg lacks a nonnegative tariff or arrival timezone is
unmapped. `arrival_timezone` can be empty: arrival timestamp is then explicitly UTC.
A missing origin timezone returns 409 `timezone_required`; there is no guessed zone.
Same-stop search is invalid. Reverse/absent sections return empty results, quote 404.
Looping trips can have repeated stop IDs: quote rejects ambiguous pairs with 409
`ambiguous_segment`, until relation-based selection is introduced. Such ambiguous pairs are excluded from search/calendar.
`sort=departure|price` (default departure), limit 1–50 (default 20). Opaque keyset
cursor is bound to origin/destination/date/sort/passengers; it is not a snapshot.
Null prices sort last. One passenger supported; values other than 1 rejected.

Calendar: `{timezone:string,month:'YYYY-MM',calculated_at:ISO,rule_version:'segment-v1',days:[{date:'YYYY-MM-DD',status:'unknown'|'no_trips',min_price_minor:number|null,trip_count:number}]}`.
Every month day is included. Month/date is interpreted in origin IANA timezone;
search and calendar exclude past departures and cancelled/completed trips.
`trip_count` counts distinct eligible trips; min price ignores missing/negative tariffs
and unmapped arrival timezones, but keeps free (zero) tariffs. No seat source exists:
`unknown` is never advertised as available/sold out. Horizon is previous month
through the next twelve months, bounded before database reads. Calendar aggregates
in PostgreSQL; it does not return/download all trips.

Errors on new endpoints: `{error:string,code:string}`; 400 `invalid_request`,
404 `not_found`, 409 `timezone_required|trip_unavailable|ambiguous_segment|already_exists|timezone_immutable|invalid_reference`,
500 `internal_error`. Geo adapter separately adds 429/503. SQL details stay private.

## Admin mapping

`GET /admin/cities` → `{items:[{id:number,name,region,timezone,popularity:number}]}`.
`POST /admin/cities` body `{name,region,timezone,popularity}` →201 city with ID.
`PUT /admin/cities/{id}` same full body →200 city. Name/region are trimmed, max
160 chars; timezone IANA; popularity nonnegative. Updating timezone is rejected to
avoid silently redating mapped stops. No city deletion cascade is provided.

Existing stops GET/POST/PUT gain nullable `city_id` and `timezone` fields.
Effective timezone = explicit stop override, else mapped city timezone. `PUT
/admin/stops/{id}` accepts `city_id:0` to clear mapping and `timezone:""` to clear
override. Omitted/null fields retain legacy partial-update semantics. Raw stop
responses return stored override; locations return effective timezone.
No addresses are parsed or auto-assigned to cities. Existing unmapped stops remain
in suggestions and legacy catalog; admin must confirm their city or timezone.
Known initial cities are catalog data only and do not invent routes.

`effective_timezone:string|null` is additionally returned by raw stop lists/details
and nested trip stop details; clients displaying legacy timelines use this field,
and explicitly label UTC when it is null.

## Optional external geosearch

`GET /geo/suggest?q=...&limit=8&lat=...&lon=...` → `{items:[{id,name,label,kind,city,region,country,latitude,longitude}]}`.
Limit 1–10, q 2–200 UTF-8 characters, optional lat/lon supplied together. Russian
language and RU country filter. IDs `osm:N:123` / W / R never become internal IDs.
Server `PHOTON_BASE_URL` must point to your instance; unset returns 503
`geo_unavailable`, with no public fallback. `PHOTON_TIMEOUT=3s` default, max 10s.
Adapter does not follow redirects; response size max 1MiB. Process-wide upstream
limit 2 requests/s, burst 10, at most four in-flight requests; overflow returns
429 `geo_rate_limited` and Retry-After. Internal locations are independent.
Multi-replica production should set a shared gateway limit before enabling geo.
