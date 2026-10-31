# Изменения для ревью

Исправления из BACKEND_AUDIT.md собраны тремя сабагентами и root в рабочей ветке
`fix/backend-audit`. Связанные изменения frontend находятся в его отдельном репозитории,
ветка `fix/sso-callback`. Этот отчёт фиксирует проверку перед переносом в `dev`. Публикация образов
настроена через GitHub Actions: `dev` → `:dev`, `main` → `:latest`, обе ветки
также получают `sha-<commit>` и метку OCI revision.

**Что изменилось**

- Доступ: управление пользователями требует admin, включая старые aliases /users;
  /users/me остаётся доступен обычному пользователю. Авторизация получает актуальную
  роль и существование пользователя из БД. LogoutAll защищён и использует claims.
- Сессии: одна атомарная ротация refresh, защита от конкурентного/повторного использования,
  синхронизация с logout/all, UTC expiry, уникальность хеша. Stale logout возвращает
  отказ вместо ложного успеха. Ошибки паролей больше не вызывают panic.
- SSO: одноразовый Redis state с привязкой к браузеру/провайдеру, PKCE Google/Yandex,
  общий timeout входа, проверка provider responses, один callback 302 с cookie.
  User+identity создаются одной транзакцией; совпадение email не связывает аккаунты.
- API: исправлены SQL рейсов и выбор по ID, ordered stops, ID сущностей,
  частичные обновления, валидация и согласованные коды HTTP. Ошибки не дописывают второй JSON
  и не раскрывают SQL. Цены на границе JSON унифицированы до float64.
- Запуск: строгая проверка конфигурации/JWT, точный CORS allowlist, HTTP port/timeouts,
  размер DB pool, graceful shutdown и остановка scheduler/БД. Recovery не пишет секреты
  из URL/headers/panic в журнал.
- Frontend: исправлено имя middleware callback и получение access через refresh + /users/me;
  защищённые запросы получают один retry после общего refresh, без рекурсивного обновления.
- CI: unit/integration/race/vet до публикации backend; синхронизированы примеры env и Compose.

**Что подтвердили проверки**

- 54 функции Go tests, включая подслучаи, с PostgreSQL и Redis:
  `GOMAXPROCS=2 go test -p 2 -race -count=1 ./...` — PASS.
  Для интеграций нужны TEST_DATABASE_URL, TEST_REDIS_ADDR, TEST_REDIS_PASSWORD.
- `GOMAXPROCS=2 go vet -p 2 ./...` и `git diff --check` — PASS.
- Frontend: `npm ci`, два теста `npm test`, `npm run build` в Node 24 Alpine — PASS.
- `docker compose --env-file .env.example config --quiet` — PASS.
- Отдельная сборка server/migrate из текущих исходников: полные goose Up и HTTP smoke
  на отдельной базе — PASS. Проверены ACL, пароль без nil-panic, CORS, state rejection,
  истечение access, refresh/replay/logout/logoutAll, SQL/ID рейсов, partial bus,
  StartTime-only update, missing-row 404 и SSO-only password login. SIGTERM дал exit 0.
- Агенты провели self-review и перекрёстное ревью. Исправлены дополнительно обнаруженные
  stale logout, конкурентная регистрация одного email, recovery dump, гонка SSO создания
  пользователя и согласование таймаутов. Тестовые серверы/базы отделены от рабочих.

**Изменения поведения, которые нужно учесть перед публикацией**

1. SSO теперь включается явно: SSO_GOOGLE_ENABLED/SSO_YANDEX_ENABLED=true.
   Все CLIENT_ID, CLIENT_SECRET и REDIRECT_URL включённого провайдера обязательны.
   Нужны FRONTEND_URL и подходящие COOKIE_DOMAIN/COOKIE_SECURE/COOKIE_SAME_SITE.
   JWT_SECRET должен содержать не менее 32 байт; HTTP_WRITE_TIMEOUT при SSO >10s.
2. Новая миграция 20261031000100_unique_refresh_token_hash отзывает дублированные
   credentials, удаляет лишние записи и вводит unique index под блокировкой таблицы.
   Затронутым пользователям потребуется новый вход. Старый код refresh несовместим
   с этим индексом; обновлять backend и схему необходимо согласованно.
3. Историческая миграция автобусов исправляет путь для ещё не применивших
   20251005230343. Goose не запускает её повторно. Применённые корректные связи
   не меняются, утраченное старой миграцией сопоставление автоматически не восстанавливается.
4. TripResponse теперь имеет явные id, bus.id, driver.id и stops[]. Потребителей
   прежнего ошибочного формата следует проверить. Новый frontend callback публикуется
   вместе с backend; адреса API и frontend должны совпадать с env и OAuth настройками.
5. Автоматического объединения аккаунтов по email нет: новый провайдер при совпадении
   email существующей учётной записи получает 409. Явный linking — отдельная задача.

**Ограничения и следующие решения**

- Реальный интерактивный вход Google/Yandex не выполнялся. Проверены протокол, мок-провайдеры,
  Redis и транзакции; нужен ручной вход после согласованного обновления образов/URL.
- Access token после logout действует до TTL. Stale logout может вернуть 401, если
  конкурентный refresh уже победил; отзыв семейства токенов потребует session/family ID.
- Для одного провайдера одновременно поддерживается один начатый вход на браузер:
  новая вкладка заменяет state-cookie, прежний callback безопасно отклоняется.
- Float64 сохраняет совместимость JSON чисел; фиксированную денежную модель и политику
  soft delete нужно согласовать отдельно. DB timestamps рейсов пока остаются без timezone.
- Зависимости не обновлялись. npm ci сообщает о существующих уязвимостях дерева frontend;
  это требует отдельного dependency review, без автоматического npm audit fix --force.
- Swagger перегенерирован перед переносом в dev: все 58 API-операций совпадают
  с маршрутизатором, схемы отражают новые DTO, aliases, cookie и SSO headers.

Итоговые изменения ветки dev относительно main доступны через
`git diff origin/main...dev` в каждом репозитории. Публикация образов выполняется
после успешных тестов GitHub Actions; локальный Compose выбирает тег через
BACKEND_IMAGE_TAG и FRONTEND_IMAGE_TAG.
