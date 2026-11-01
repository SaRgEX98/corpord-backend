# Реальная геоподложка пилота: Екатеринбург

Проверено 2026-11-01. Это отдельный локальный стенд: существующий
`corpord-infra`, production/dev-контейнеры и их окружение не изменяются.
Подложка содержит реальные улицы, здания, воду и подписи, а не только background.
Область `[60.45, 56.72, 60.75, 56.92]`, уровни 0–14; за пределами области
покрытие не обещается. Москва и полные направления требуют отдельных экстрактов.
Линия между остановками сама по себе не означает вычисленный автомобильный маршрут.

## Источник и лицензии

Официальная документация Protomaps разрешает частичное скачивание посредством
`pmtiles extract`: [downloads](https://docs.protomaps.com/basemaps/downloads),
[CLI](https://docs.protomaps.com/pmtiles/cli). Использован HTTP Range к
`https://build.protomaps.com/20261031.pmtiles`; планета не скачивалась.
Сервер возвращал `Accept-Ranges: bytes`, ETag
`64471e67402a329fd7a9614f56b14a57-519`. Metadata: schema 4.15.2,
OSM replication `2026-10-31T04:00:00Z`. Build-архивы могут удаляться по retention
источника; сохраните проверенный городской архив и manifest в собственном хранилище.
Если исходный URL удалён, скрипт завершится ошибкой: замена снимка требует нового
ревью, контрольных сумм и проверки качества, публичного runtime fallback нет.

Закреплены go-pmtiles 1.31.2, `@protomaps/basemaps` 5.7.2 (русские подписи),
basemaps-assets commit `028c18f713baecad011301ff7a69acc39bcc2ae7`.
Скрипт проверяет SHA256 инструмента/asset ZIP/регионального архива и npm integrity.
`map-manifest.json` содержит SHA256/размеры всех файлов, которые обслуживает карта.

Лицензии сохраняются рядом с данными в `licenses/`: стили BSD-3-Clause,
Noto Sans Regular/Medium/Italic SIL OFL, иконки MIT (Mapzen).
Данные OSM/водные полигоны — ODbL, Natural Earth — public domain,
landcover — ESA WorldCover CC BY 4.0. Полный upstream перечень —
`licenses/basemaps-DATA.md`. Attribution OSM/Protomaps/ESA WorldCover встроена
в оба style JSON и должна оставаться видимой в MapLibre.

## Повторная подготовка и запуск

Требуются Linux x86_64, Python 3.11+, Node (рекомендуется используемый проектом
Node 26), npm, Docker Compose. Данные/бинарники/индексы не добавляются в Git.
Команды выполнять из корня backend checkout; путь данных должен быть вне checkout.

```bash
python3 deploy/geo/pilot/map-prepare.py /absolute/outside-git/maps
MAP_DATA_DIR=/absolute/outside-git/maps docker compose -f deploy/geo/pilot/map-compose.yaml config --quiet
MAP_DATA_DIR=/absolute/outside-git/maps docker compose -f deploy/geo/pilot/map-compose.yaml up -d
```

Пилот обслуживается на `127.0.0.1:8092`. Все URL в стилях абсолютные и локальные:

```dotenv
NUXT_PUBLIC_MAP_STYLE_URL=http://127.0.0.1:8092/styles/corpord-light.json
NUXT_PUBLIC_MAP_DARK_STYLE_URL=http://127.0.0.1:8092/styles/corpord-dark.json
```

Разрешены frontend origins `http://localhost:3000/3100/3200` и соответствующие
`127.0.0.1`. CORS не выдаётся произвольным origins. Для другого адреса нужно
изменить allowlist и повторно сгенерировать стили с `--origin`, затем проверить
все browser requests. Nginx позволяет только стили, шрифты, спрайты, лицензии,
manifest и городской архив; инструменты и preparation logs недоступны по HTTP.
Для HTTPS нужен собственный HTTPS origin, без mixed content.

Nginx закреплён digest из предыдущего geo-этапа, работает UID 101 с read-only FS,
128MiB/1 CPU, отдельным Compose project `corpord-map-pilot`. Объём сохранённых
данных включает инструменты, ZIP, npm и все диапазоны glyphs; не только архив.

```bash
MAP_DATA_DIR=/absolute/outside-git/maps docker compose -f deploy/geo/pilot/map-compose.yaml down
```

## Проверенные результаты

На локальном стенде данные лежат в `corpord-geo-pilot-data/maps` вне Git:

| Проверка | Результат |
| --- | --- |
| Региональный PMTiles | 6 462 836 bytes, 360 MVT tiles, z0–14 |
| SHA256 PMTiles | `f86daa943da333c07ba8b1270adb64bf74b8804030136bb292c0c4e3c67ec31f` |
| PMTiles extract | 42 HTTP requests, 6.7MB transfer; 27.83s, max RSS 60 900KiB |
| PMTiles verify | PASS |
| Asset ZIP | 6.4MiB, все три Noto Sans fontstack, полный набор диапазонов |
| Сохранённые данные | ~101MiB с инструментами/asset ZIP/npm; лимит 2GB соблюдён |
| Общая загрузка подготовки | <40MB, включая инструмент/assets/npm/extract; лимит 300MB соблюдён |
| nginx idle RSS (docker stats) | ~2.15MiB из лимита 128MiB |
| HTTP / оба стиля | HEAD, Range206/416, ETag304, CORS allow/deny, glyphs1024–1279 всех трёх шрифтов и sprite JSON/PNG — PASS независимого geo quality harness |

Шрифты, стили, спрайты и PMTiles обслуживаются локально; внешние ссылки attribution
не являются сетевыми запросами карты. Цвета background/foreground/buildings взяты
из существующих токенов Corpord, отдельно для светлой и тёмной темы; семантические
цвета парков/воды/дорог остаются картографическими. Русские названия используют
`lang: ru` с правилами fallback официальной библиотеки, качество каждого объекта
зависит от данных OSM.

Raw logs/resource metadata и полный asset manifest остаются рядом с данными.
Проверки пикселей/дорог/русских подписей в настоящем frontend выполняются отдельно
браузерным интеграционным тестом; одна HTTP-проверка этого не доказывает.
Это измерения городского пилота, а не sizing Урала/России или оценка Photon.

Для обновления: подготовить новую датированную директорию, проверить исходные
лицензии, hashes, bounds, качество обеих тем и все runtime origins, затем перевести
style URLs на новый immutable набор. Старый набор оставить до истечения cache.
Rollback — вернуть прежние style URLs и сохранённую директорию; файл с прежним
именем не заменять, пока старые клиенты читают Range.
