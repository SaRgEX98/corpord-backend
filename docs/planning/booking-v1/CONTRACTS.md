# API MVP v1

Базовый путь `/api/v1`. Все timestamps RFC3339 UTC; integer `amount_minor`, RUB.
Ошибка `{error: string, code: string}`. Пользователь берётся из middleware, не body.
Чужая запись404; отсутствие auth401; admin ACL403. Не логировать passenger payload.

## Бронирование

- Public `GET /trips/:id/availability?boarding_stop_id=N&alighting_stop_id=N`:
  `{trip_id,boarding_stop_id,alighting_stop_id,is_demo,bookable,status,seats,hold_seconds}`.
  `status`: available/sold_out/unknown; seats integer|null, hold_seconds600 — максимальный TTL.
  Фактическое истечение ограничено отправлением; авторитетен `Booking.expires_at`.
  Реальные рейсы unknown/bookable=false. Endpoint не изменяет БД.
- Auth `POST /me/bookings`, header `Idempotency-Key` (8..128 ASCII alnum/_/-):
  `{trip_id,boarding_stop_id,alighting_stop_id,passenger_id? ,passenger_name?}`.
  Ровно одно из passenger_id или непустого passenger_name. Одна персона, no quantity.
  Ответ201 (повтор200): Booking. Цена/seat/owner из сервера, не из body.
- Auth `GET /me/bookings?limit=30&cursor=...`: `{items:Booking[],next_cursor:string|null}`.
- Auth `GET /me/bookings/:id`: Booking.
- Auth `POST /me/bookings/:id/cancel`: Booking; повтор отмены идемпотентен.

Booking:
```
{id,trip_id,boarding_stop_id,alighting_stop_id,boarding_name,alighting_name,
 departure_time,arrival_time,timezone,arrival_timezone,passenger_name,
 status:"held"|"cancelled"|"expired",seat_number:number,
 amount_minor:number,currency:"RUB",is_demo:boolean,
 created_at,expires_at,cancelled_at:string|null}
```
Не отдавать idempotency key/hash, user_id и внутренние snapshot/provenance.
Основные409: sold_out,hold_limit,idempotency_conflict,trip_unavailable,
inventory_unavailable,ambiguous_segment. Timeout503, validation400. Ограничение hold TTL не продлевается
повтором того же idempotency key. Старые search/calendar/quote контракты не меняются.

## Пассажиры

Auth `GET /me/passengers?limit=30&cursor=...`: `{items:Passenger[],next_cursor}`.
`POST /me/passengers` body `{full_name}` →201 Passenger.
`PUT /me/passengers/:id` body `{full_name}` →200 Passenger.
`DELETE /me/passengers/:id` →204. Имена trim,1..160 Unicode chars, без control chars.
Passenger `{id,full_name,created_at,updated_at}`. Отсутствующий/чужой ID404.
Cursor scoped owner. Удаление не удаляет и не переписывает snapshot имени в бронях.

## Качество справочника

Все ниже только admin.
- `GET /admin/catalog-quality/summary`:
  `{unmapped_stops:number,protected_unmapped_stops:number}`; bounded aggregate,
  не грузить все записи/массовые кандидаты в UI.
- `GET /admin/catalog-quality/stops?q=&limit=30&cursor=...`:
  unknown city stops по ID, `{items:QualityStop[],next_cursor}`.
  QualityStop `{id,name,address,latitude,longitude,city_id:null|number,
  timezone:string|null,city_name:string|null,region:string|null,
  version:string,has_trips:boolean}`.
- `POST /admin/catalog-quality/stops/:id/mapping`:
  `{city_id:number,version:string,reason:string}`. Только явный выбранный city ID;
  причина trim5..500 chars. Ответ200 QualityStop (новая version).
  409 stale_version/stop_in_use/timezone_conflict,404 неизвестный stop/city.
  Transaction locks stop/city; запрет менять effective timezone используемых рейсов.
  Не менять source_snapshot: ручная правка должна защищаться повторным импортом.
- `GET /admin/catalog-quality/stops/:id/history?limit=20`:
  `{items:[{id,actor_id,created_at,reason,before,after}]}`. Максимум100, без full-table.
  before/after — `{city_id,timezone}`. Нет account names/emails/password hashes.
- Города выбираются существующим `/catalog/cities?q=&limit=30&cursor=...`.

Wiring root: agent booking экспортирует `NewBooking(*sqlx.DB)` и
`Register(public,authorized *gin.RouterGroup)` + bounded expiry task API.
Agent quality экспортирует `NewCatalogQuality(*sqlx.DB)` и
`Register(admin *gin.RouterGroup)`; интерфейсы согласовать сообщением до завершения.
