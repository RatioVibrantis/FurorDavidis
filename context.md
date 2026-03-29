# FUROR DAVIDIS - Context (CTS)
> "Ira Davidis" / "Furor Davidis" - ярость Давида против наблюдателя

---

## [META]
- Version: v0.6.9-dev
- Date: 2026-03-28
- Status: Encoding cleanup in logs + RAG timeout scoring fix + fresh build complete
- Source of truth: ONLY `context.md` (вместо `claude.md`)
- Relation: standalone, работает рядом с AWG (Vanus Scrutator или любой AWG-клиент)

---

## [PROTOCOL]
- Формат: CTS (теги, стрелки, статусы, короткие блоки без воды)
- Язык: RU
- Тон: технический, прямой
- Если задача ясна: act mode (план + выполнение без лишних переспросов)
- При крупных изменениях: обязательно обновлять этот файл

---

## [AUTHOR]
- Role: Lead Specialist / DevOps / NetOps (не программист)
- Mode: вайбкодинг (архитектура от автора, реализация от ИИ)

---

## [CONCEPT]
```
Goal: AI-оркестратор трафика
  -> создает видимость обычного браузинга снаружи AWG-туннеля
  -> путает DPI-классификаторы провайдера
  -> асимметричная война: их ИИ-кластер vs локальные 1.5B параметры
```

---

## [ARCH]
```
[Monitor] -> AWG iface stats (RTT trend / bytes/s / session_age / hour)
    ->
[Rules Engine] (Go, детерминированный)
    RTT +20% за 10min -> trigger
    session > 30min -> HotSwap
    low_traffic -> inject cover
    ->
[AI Engine] <- LM Studio HTTP:1234
    input: profile + hour + sites (<=40 токенов)
    output: JSON [{url, read_sec}, ...]
    модель: Qwen2.5-1.5B-Instruct (recommended)
    ->
[Cover Executor] (Go net/http + utls)
    bind -> физический адаптер IP (bypass AWG)
    fallback без bind если WFP блокирует
    реальные TLS/HTTPS запросы к легитимным сайтам
    User-Agent: Chrome 120, cookiejar сессии
    timing: read_sec + jitter +-20%
    ->
[HotSwap Controller]
    SSH -> VPS -> reload xray config
    ротация: microsoft.com -> youtube.com -> github.com
```

Параллельно:
```
AWG tunnel (amneziawg.exe, внешний процесс)
UDP -> VPS : 40000-55000
```

На сервере:
```
VPS (Ubuntu)
AWG endpoint (UDP)
xray DECOY (TCP:443)
  observer probe -> TLS -> redirect legit domain
  не туннель, только маскировка сервера
```

---

## [AI BACKEND] - LM Studio (единственный)
- Backend: `LM Studio` (`lmstudio.ai`)
- API: OpenAI-compatible, `localhost:1234`
- Method: `POST /v1/chat/completions`

Recommended model:
- `lmstudio-community/Qwen2.5-1.5B-Instruct-GGUF`
- Альтернативы: любая instruct-модель (Qwen3-1.7B, Mistral-7B и т.д.)

Autoload модели:
1. `GET /v1/models` - проверка загруженной модели
2. Если пусто -> `POST /api/v0/models/load {identifier: ...}`
3. Ожидание до 60с появления в `/v1/models`
4. Если не появилась: AI отключается, cover продолжает работать по rules

Prompt (compact, <=40 токенов):
- System: `Output ONLY a JSON array of URLs. No text, no markdown.`
- User: `Generate N URLs for [profile] user at HH:00. Sites: [...]. JSON only: [...]`
- `N = session_min/4`, минимум 3, максимум 8
- Sites: адаптивно 5..12 из списка (`siteLimit = clamp(session_min/3, 5..12)`)

EngineConfig:
- `EngineConfig { LMStudioModel string }` (`""` = `DefaultModel`)

Stop/Backend behavior:
- LM Studio не останавливаем (это пользовательский системный процесс)
- `Backend()` возвращает `"lmstudio"`, если backend ready

---

## [COVER TRAFFIC]
```
Физадаптер (Wi-Fi/Eth):
  -> UDP:4XXXX -> VPS        (AWG, реальный шифрованный трафик)
  -> TCP:443   -> youtube.com (cover, реальный TLS снаружи AWG)
  -> TCP:443   -> wikipedia.org
  -> TCP:443   -> github.com
  -> TCP:443   -> docs.python.org
```

Что видит ISP/DPI:
- обычный пользователь + некоторый UDP
- класс: ~60% web browsing + ~40% unknown UDP -> безопасный профиль

Route bypass (cover/executor.go):
1. `ParseGateway()` -> `physGW`
2. Resolve hostname -> IP
3. `route add <cover_IP>/32 <physGW>`
4. HTTP(S) запрос без bind
5. Ядро роутит трафик через физадаптер по /32 маршруту

Важно:
- `bind(physIP)` может падать с `WSAEACCES` (WFP)
- `connect` без bind WFP обычно не блокирует
- `/32 route` обходит AWG `0.0.0.0/1` и `128.0.0.0/1`

---

## [BEHAVIOR PROFILES]
- Поведенческие списки стали именованными и редактируемыми (`cover_lists`)
- В каждом профиле хранится собственный набор списков + активный список (`active_cover_list_id`)
- При переключении профиля UI подгружает именно его активный список доменов
- Для legacy-профилей выполняется автоперенос из `CoverSites/BehaviorProfile` в `cover_lists`
- HotSwap `decoy_domains` синхронизируется с активным AI Cover списком (по доменам из URL)

---

## [HOTSWAP DOMAINS]
- Decoy domains:
  `microsoft.com, youtube.com, github.com, cloudflare.com, apple.com, linkedin.com`
- Rotation: по команде или по таймеру (default 30min)
- Mechanism: `SSH -> sed xray.json -> reload/restart xray`

---

## [STRUCT] реализовано
- `main.go` - Wails entry point
- `app.go` - биндинги Start/Stop/Deploy/Connect/Diag/Profile/Memory + multi-profile API (`List/Create/Select/Delete/Import/Export profile`)
- `go.mod` - module `github.com/yourorg/furor-davidis`, Go 1.22
- `go.work` - workspace (Vanus + FurorDavidis)
- `wails.json` - no npm build (vanilla JS, `dist/`)
- `build/windows/wails.exe.manifest` - `requireAdministrator = true` (UAC)

`internal/`:
- `logger/logger.go` - Entry struct, EventsEmit в UI
- `profile/store.go` - Profile JSON: VPS, AWG, LMStudioModel, cover, hotswap
- `profile/store.go` - multi-profile storage (`profiles[]` + `active_profile_id`), migration from legacy single-profile format, profile import/export
- `profile/store.go` - v0.6.4 data model: `servers[]` + `active_server_id`, each server has `clients[]` + `active_client_id`; runtime merged profile preserved for rules/deploy/connect
- `memory/store.go` - RAG: Record -> Evaluate -> BuildPromptContext
- `memory/store.go` - RAG: zero/missing RTT samples are marked as failure with penalty (not neutral), notes/log strings normalized to ASCII
- `routing/windows.go` - ParseGateway()/ParseGatewayExclude() из Vanus
- `monitor/awg.go` - ping RTT, RTT trend
- `ai/engine.go` - LM Studio HTTP client, compact prompt
- `cover/executor.go` - net/http + utls, WFP-aware fallback, gateway exclude AWG iface
- `rules/engine.go` - детерминированные триггеры, orchestration, pass AWG iface to cover executor, active cover list resolver
- `diag/checker.go` - CheckLocal(model): AWG files + LM Studio status
- `ssh/client.go` - SSH Dial/RunScript
- `payload/scripts.go` - DeployScript/HotSwapScript/ClientConfig templates
- `payload/scripts.go` - deploy/hotswap script output normalized to clean ASCII logs (no mojibake markers)
- `deploy/orchestrator.go` - Deploy/HotSwap/Verify
- `connect/manager.go` - ConnectAWG/Disconnect/addAWGRoutes
- `connect/manager.go` - PowerShell runner now forces UTF-8 output encoding before command execution

`frontend/dist/`:
- `index.html` - 6 вкладок
- `style.css` - dark theme
- `main.js` - Wails bindings + UI logic, LM Studio models refresh/select, profile manager UI, cover-list manager UI, UI i18n dictionary (`ru`/`en`) + runtime switch
- `main.js` - settings manager switched to 2-level selectors: `Server` + `Client` with CRUD actions per level
- `main.js`/`style.css` - adaptive live block получил цветовые индикаторы состояния (enabled/mode/rag/timeout-policy)
- `main.js` - синхронизация языка UI с системным треем (`SetUILanguage`), AI статус в header: статичная метка `AI` + цветовой индикатор

`root`:
- `tray.go` - системный трей (сворачивание в трей, меню `Libertad/Libero`, `Open/Открыть`, `Exit/Выход`, иконка gray/green)

`app.go`:
- `GetLMStudioModels()` - отдает список загруженных моделей LM Studio для UI
- `ListProfiles()/CreateProfile()/SelectProfile()/DeleteProfile()` - управление профилями
- `ExportActiveProfile()/ImportProfile()` - импорт/экспорт профилей через file dialog
- `ListServers()/CreateServer()/SelectServer()/DeleteServer()` - управление VPS-сущностями
- `ListClients()/CreateClient()/SelectClient()/DeleteClient()` - управление клиентскими профилями внутри активного VPS

---

## [PROFILE] ключевые поля
```go
ID, Name
VPSHost, VPSPort, VPSUser, VPSPassword
Deployed, AWGListenPort
AWGClientPrivKey, AWGClientPubKey, AWGServerPubKey, AWGClientConfig
AWGExePath, AWGInterface
DecoyDomains, HotSwapEnabled, HotSwapInterval
LMStudioModel string `json:"lmstudio_model"`
CoverLists []CoverList, ActiveCoverListID string
CoverSites, BehaviorProfile // legacy-compat mirror of active list
Intensity, SessionMinutes
```

Новый контейнерный формат:
```json
{
  "active_server_id": "...",
  "servers": [
    {
      "id": "...",
      "name": "...",
      "clients": [
        {"id": "...", "name": "..."}
      ],
      "active_client_id": "..."
    }
  ]
}
```

---

## [KEY DECISIONS]
| Компонент | Решение | Причина |
|---|---|---|
| AI runtime | LM Studio HTTP:1234 | GUI-каталог моделей, без subprocess |
| AI prompt | <=40 токенов | малые модели хуже на длинном контексте |
| Cover bind | physIP + fallback без bind | bypass AWG, устойчивость к WFP |
| DNS cover | custom Resolver removed | снизили сложность, вернуть при DNS leak |
| AI task | генерирует URL sequence, НЕ триггеры | LLM силен в генерации траекторий |
| Rules | детерминированный Go | надежнее для trigger-логики |
| Server xray | decoy only, не туннель | упрощение, меньше отказов |
| AWG | внешний процесс | не ломаем рабочий компонент |
| GUI | Wails v2 | меньше ресурсов, современный UI |
| physIP | ParseGateway() + route print | проверено в Vanus/Furor |
| UAC | requireAdministrator | route add + tunnel service требуют admin |
| Код-правило | полный файл, не фрагменты | снижает риск потери контекста правок |
| Безопасность | close handles, чистка секретов в памяти где возможно | базовая operational hygiene |

---

## [DEPLOY] зафиксированные решения
- VPS: Ubuntu 24.04 (noble), тест пройден
- AWG PPA: `ppa:amnezia/ppa` (НЕ `amneziavpn/ppa`)
- AWG server config: без `DNS=` в `[Interface]` (иначе ломается DNS VPS)
- Docker install: `curl -fsSL https://get.docker.com | sh`
- xray image: `teddysun/xray:latest`, `dokodemo-door` TCP:443 -> DecoyDomain
- Install order: `[1] apt [2] AWG [3] AWG-conf [4] Docker [5] xray [6] UFW [7] fail2ban`

Ловушка:
- `DNS=1.1.1.1` в серверном конфиге AWG -> ломает DNS VPS -> `curl (6)`
- Решение: DNS оставить только в `ClientConfig`

---

## [CONNECT] Windows AWG client
- `amneziawg.exe /installtunnelservice furor.conf`
- Создает `WireGuardTunnel$furor`
- Имя интерфейса = имя `.conf` без расширения

Требования:
- запуск приложения от администратора
- `build/windows/wails.exe.manifest`: `requestedExecutionLevel = requireAdministrator`
- `wintun.dll` рядом с `amneziawg.exe`

Anti-loop route:
- до подъема AWG: `route add <VPS_IP>/32 <physGW>`
- далее AWG default split routes: `0.0.0.0/1` + `128.0.0.0/1`

---

## [BUGS FIXED]
1. `Start()` вызывался 5 раз -> guard `a.running=true` до blocking call
2. `ppa:amneziavpn/ppa` -> 404 -> заменен на `ppa:amnezia/ppa`
3. DNS VPS ломался после AWG старта -> убран `DNS=` из server config
4. `curl: (6)` на Docker install -> следствие бага #3
5. `furor_awg.conf` -> интерфейс `furor_awg` != `furor` -> rename в `furor.conf`
6. `route add` / `installtunnelservice` exit 1 -> не было admin manifest
7. UI log window рос бесконечно -> фиксированная высота
8. `llamafile` timeout/нестабильность -> удален, перешли на LM Studio only
9. Cover `WSAEACCES (10013)` при bind physIP -> fallback без `LocalAddr`
10. AI timeout 60s -> prompt сокращен до ~40 токенов
11. Cover не уходил наружу AWG (`connectex ... forbidden`) -> в cover добавлен `ParseGatewayExclude(AWG iface)` + явный gateway log
12. Rules не передавал AWG iface в cover executor -> добавлен `SetAWGInterface(p.AWGInterface)`
13. LM Studio выбирал "первую модель в списке" -> явный `LMStudioModel` теперь приоритет; добавлена загрузка/ожидание именно выбранной модели
14. В Settings добавлен опрос LM Studio и dropdown выбора модели (`GetLMStudioModels` + refresh/select в UI)
15. DNS leak через локальный роутер (`192.168.0.1:53`) -> добавлен `dnsmasq` на VPS (`10.8.0.1`), клиентский DNS в AWG конфиге переведен на VPS DNS, в `connect.Manager` добавлен `normalizeDNS()` для принудительной фиксации `DNS = 10.8.0.1`
16. Windows продолжал использовать DNS роутера даже с `DNS=10.8.0.1` в конфиге AWG -> в `connect.Manager` добавлен принудительный DNS-policy на `Connect`: `Set-DnsClientServerAddress` для `furor` + NRPT rule `.` -> `10.8.0.1`; на `Disconnect` policy удаляется и DNS интерфейса сбрасывается
17. Cover мог деградировать после DNS anti-leak из-за fallback на системный resolver -> `internal/cover/executor.go` переведен в DoH-only режим (без `net.DefaultResolver`) + multi-DoH failover (Cloudflare/Google/Quad9); fallback TLS-клиент тоже использует DoH-only
18. Убран риск DPI-сигнатуры Go TLS: в cover удален fallback на стандартный `crypto/tls`; теперь `DialTLSContext` работает только через uTLS с ротацией профилей (`HelloChrome_Auto`, `HelloFirefox_Auto`, `HelloEdge_Auto`) и жестким fail при неуспехе всех профилей
19. Убран глобальный route-add для DoH DNS IP (например `1.1.1.1`) в cover; route-add остается только для целевых IP маскировочных доменов
20. AI cover-промпт переведен на сессионную модель поведения: теперь JSON-объекты включают `url`, `referer`, `read_sec`; в `executor.Run` добавлено использование `item.referer` (с fallback на предыдущий URL) для более реалистичных цепочек переходов
21. В `internal/cover/executor.go` внедрены доменные мини-сессии: `Run()` группирует подряд идущие URL по хосту и выполняет их через один `http.Client/Transport` (keep-alive), что уменьшает число TLS handshakes и делает паттерн ближе к браузерному
22. Для диагностики `EOF` в cover увеличены route/handshake задержки до `500ms` (перед dial и между uTLS-профилями), чтобы исключить гонки применения `/32` маршрутов в Windows стеке
23. В cover временно был включен режим `HTTP/1.1 only`: `NextProtos={"http/1.1"}`, `ForceAttemptHTTP2=false`, `TLSNextProto` отключен; это убрало часть `EOF`, но выявило новый симптом
24. Новый симптом после успешного handshake: `malformed HTTP response "\\x00\\x00..."` (бинарные HTTP/2 SETTINGS-фреймы, когда клиент ожидает HTTP/1.1)
25. По результатам логов `Rejecting ALPN "h2"` стало ясно, что крупные сайты (Yandex/Kinopoisk/Wikipedia) часто настаивают на h2 для современного fingerprint
26. Перевод cover в Hard Mode (h2-ready): убран reject `h2`, возвращен ALPN `{"h2","http/1.1"}`, включен `ForceAttemptHTTP2=true`, добавлен `http2.ConfigureTransport(transport)` в `buildClient`
27. TLS для cover сейчас: `MinVersion=tls1.2`, `MaxVersion=tls1.3`; профили в приоритете: `HelloChrome_102`, `HelloFirefox_105`, затем `HelloChrome_120`, `HelloFirefox_120`
28. Сборка после h2-патча подтверждена: `go test ./...` OK, `wails build` OK, артефакт `build/bin/FurorDavidis.exe`
29. По свежему `furor_debug.log` выявлено: несмотря на h2-ready флаги, оставалась ошибка `net/http: HTTP/1.x transport connection broken: malformed HTTP response "\\x00\\x00..."` на `https://kinopoisk.ru/` (сервер слал HTTP/2 SETTINGS, а запрос обрабатывался h1-путем)
30. В `internal/cover/executor.go` внедрен dual transport для cover-клиента:
   - новый `coverTransport` (custom `RoundTripper`)
   - приоритет: сначала `http2.Transport` (через тот же uTLS `DialTLSContext`), затем fallback на `http.Transport` (strict h1) при протокольной несовместимости
   - h1 транспорт зажат: `ForceAttemptHTTP2=false` + `TLSNextProto` пустой map
31. Для `http2.Transport` добавлена совместимая обертка `DialTLSContext(ctx, network, addr, *tls.Config)` -> reuse существующего `dialTLSWithUTLS`
32. После dual-transport фикса компиляция подтверждена: `go test ./...` OK и `wails build` OK (новый `build/bin/FurorDavidis.exe`)
33. Добавлена поддержка нескольких профилей VPS/AWG/AI: `profiles[]` + `active_profile_id`, автозагрузка последнего активного профиля при старте
34. Добавлены операции управления профилями в UI/Backend: create/select/delete/import/export; при switch профиля выполняется безопасный stop AI + disconnect AWG
35. AI Cover переведен с hardcoded `developer/casual/researcher` на редактируемые именованные списки (`cover_lists`), с привязкой активного списка к профилю и миграцией legacy-полей
36. Обновлен UI text-layer: исправлена битая кодировка в `frontend/dist/index.html`, `main.js`, `style.css`
37. Сборка от 2026-03-28 выполнена через `wails build` без `-clean`: перезаписан только `build/bin/FurorDavidis.exe`, остальные файлы в `build/bin` сохранены
38. Добавлен двуязычный интерфейс пользователя (`RU/EN`) с переключением в UI и сохранением выбора языка в `localStorage` (`furor_ui_lang`), без изменений backend-логики
39. Обновлены action-label кнопок: deploy = `¡Viva la libertad`; connect = `Libertad` (idle) / `Libero` (connected)
40. Переход на 2-level профильную модель: отдельные VPS-сущности (`servers`) и клиентские профили (`clients`) внутри каждого VPS; добавлена миграция из `profiles[]` и legacy single-profile
41. UI Settings переделан под иерархию `Server/Client`: отдельные select/create/delete для серверов и клиентов, с авто-перезагрузкой активной конфигурации
42. Фикс резкого закрытия приложения после миграции: устранена рекурсивная инициализация в `internal/profile/store.go` (`mergeToProfile`/`withDefaults`), после фикса `go test ./...` и `wails build` проходят стабильно
43. Перед первым деплоем добавлен guard-подсказка: предложить пользователю проверить AI Cover список; при отмене — переход на вкладку AI Cover
44. Синхронизация маскировки: активный AI Cover список теперь автоматически задает `decoy_domains` для HotSwap (единый источник доменов)
45. AI prompt site-cap переведен с фиксированного `5` на адаптивный `5..12` в зависимости от длительности сессии (`session_min/3`, clamp 5..12)
46. RAG memory: оценка `RTT before/after` отвязана от cancel cover-сессии; запись теперь дооценивается отдельной goroutine даже при частых re-trigger
47. RAG memory: `RTT` для оценки переведен на медиану из 3 ping-замеров (`MeasureRTTMedian`) вместо одиночного значения
48. RAG memory: таймауты оценки больше не теряются — timeout помечается как `OutcomeFailure` + `eval_timed_out=true` + мягкий штраф score
49. Добавлен warmup-гейт памяти: `BuildPromptContext` не инжектирует RAG в prompt, пока не набрано минимум `10` оцененных записей
50. Добавлена A/B-настройка штрафа timeout в профиле клиента: `memory_timeout_policy = low/base/high` (`0.10/0.15/0.20`), управление из UI Settings
51. В UI Memory добавлен отдельный счетчик `Timeouts`; в Settings добавлен переключатель `RAG timeout penalty` (RU/EN локализация)
52. В AI Cover добавлен live-блок адаптива (`Adaptive live`) с визуальными цветовыми индикаторами: enabled/mode/rag-weight/timeout-policy
53. В deploy/connect логи добавлены тематические маркеры: `¡Viva la libertad`, при установке сервера `Launching Supremo...`, при успешном подключении `Libero`
54. README/README.ru синхронизированы с фактической архитектурой (LM Studio only, без legacy llamafile), обновлены quick-start/settings/features
55. Реализован системный трей для Windows: закрытие окна уводит приложение в трей, добавлены пункты `Libertad/Libero`, `Open/Открыть`, `Exit/Выход`, иконка состояния (gray/green)
56. Добавлена live-синхронизация языка трея с UI (`EN/RU`) через backend метод `SetUILanguage`
57. Исправлена блокировка кнопки Connect на вкладке подключения: кнопка доступна в idle-состоянии, backend при connect авто-стартует AI Cover
58. Добавлен gate готовности LM Studio перед подключением AWG: если LM не поднят/модель не загружена, connect прерывается с явной подсказкой пользователю
59. UI тексты AI обновлены: статус в header теперь статичный `AI` (состояние отражает цвет индикатора); в подсказке AI backend зафиксированы рекомендации по моделям (Qwen3-1.7B / instruct / 0.6B для слабых ПК)
60. Исправлен источник «абракадабры» в deploy-логах: `internal/payload/scripts.go` переписан на clean ASCII runtime output (`-- [step] ...`, `->`, без поврежденных unicode-последовательностей)
61. Исправлен RAG edge-case: при `rtt_before<=0` или `rtt_after<=0` запись больше не остается `neutral`; теперь это `failure` с `score`-штрафом и пометкой `missing RTT sample`
62. Нормализованы текстовые note-поля RAG (`RTT a->b`, `stable`) и prompt-summary маркеры (`[OK]/[BAD]`) для снижения риска кодировочных артефактов в JSON/логах
63. В `connect.runPS` добавлена принудительная установка `OutputEncoding=UTF-8`, чтобы PowerShell вывод логировался в предсказуемой кодировке
64. Сборка от 2026-03-28 подтверждена: `go test ./...` OK, `wails build` OK, обновлен `build/bin/FurorDavidis.exe` без очистки `build/bin`
65. При `Load()` памяти добавлена авто-нормализация legacy-записей: `neutral` + `rtt_after<=0` теперь переводится в `failure/eval_timed_out` с мягким штрафом score
66. Скорректирована семантика RAG для стабильных каналов: кейс `RTT before == RTT after` теперь трактуется как слабый `success` (не `neutral`), чтобы память не «застывала» на low-latency линках и могла накапливать сигнал

---

## [GOTCHAS]
- Cover bind `physIP` может блокироваться WFP на этапе bind
- Без `/32` route cover уйдет в AWG tunnel из-за `0.0.0.0/1` и `128.0.0.0/1`
- xray decoy слушает `TCP:443`, не AWG UDP
- HotSwap через restart дает обрыв 3-5s, лучше `SIGHUP` где возможно
- Go `crypto/tls` != Chrome fingerprint, использовать `utls`
- AI inference serial: не запускать новый inference до завершения предыдущего
- Малые модели (<2B): длинный prompt деградирует качество, держать 30-50 токенов
- LM Studio `/api/v0/models/load` грузит только уже скачанную модель (не скачивает)
- `wails build -clean` очищает `build/bin` (удаляет рабочие логи/конфиги). Для обычной итерации сборки использовать `wails build` без `-clean`
- `logger.Logger`: нет `Warnf`, использовать `Infof("WARN: ...")`
- После HotSwap делать post-verify `TCP:443`; при fail -> WARN, AWG продолжает
- Для проверки DNS leak смотреть не только обычный DNS/53, но и mDNS/LLMNR/WS-Discovery локальные broad/multicast пакеты (это отдельный класс локального шума, не обязательно leak туннельного DNS)
- Симптом `malformed HTTP response "\\x00\\x00..."` в cover почти всегда означает рассинхрон h2/h1 (сервер уже говорит HTTP/2, а транспорт ожидает HTTP/1.1)
- Если `Rejecting ALPN "h2"` появляется массово на современных доменах, режим strict HTTP/1.1 становится нежизнеспособным — нужен полноценный h2-путь (сейчас уже внедрен через `http2.ConfigureTransport`)
- Даже при `ForceAttemptHTTP2=true` и `http2.ConfigureTransport` возможно попадание в h1-path при кастомном uTLS-dial; для устойчивости нужен явный dual transport (h2-first + h1-fallback) — теперь реализовано
- В UI можно добавить много сайтов в cover list; AI-подсказка берет адаптивно первые `5..12` сайтов (в зависимости от `session_minutes`)
- На «холодном старте» RAG-памяти (мало оцененных записей) prompt-контекст намеренно пустой до достижения warmup-порога (`10`), это нормальное поведение
- На очень стабильных каналах (например, `1ms -> 1ms`) записи теперь получают слабый `success`, иначе RAG может годами оставаться в `neutral` и не обучаться
- `Timeouts` в Memory UI — отдельный KPI качества телеметрии/оценки; при частых timeout стоит проверять стабильность RTT-замеров и сетевые условия VPS
- A/B политика `memory_timeout_policy` влияет только на штраф за timeout (`low/base/high = 0.10/0.15/0.20`) и не меняет базовую логику success/failure по delta RTT
- Live-цвета в `Adaptive live` отражают состояние параметров, но не влияют на decision-логику rules engine (только визуальный слой)
- В текущей логике connect требует готовый AI backend (LM Studio + loaded model): при неготовности connect блокируется понятной ошибкой, чтобы не было ложного «подключено»

---

## [STAGES]
- `[v0.1] CONCEPT` - done
- `[v0.2] SCAFFOLD` - done
- `[v0.3] MODULES` - done
- `[v0.4] DEPLOY` - done
- `[v0.5] AI+COVER` - done
- `[v0.6] MONITOR` - in progress (сбор и нормализация AWG metrics)
- `[v0.7] MEMORY` - in progress (RAG loop стабилизирован: independent eval, median RTT, timeout-policy, warmup gate)
- `[v0.8] HOTSWAP` - done (SSH + xray HotSwap + post-verify)
- `[v1.0] RELEASE` - pending (сборка, README, дистрибутив)

---

## [NEXT TASKS]
- стабилизировать monitor pipeline: sampling, smoothing, trigger quality
- прогнать memory/RAG цикл под реальной нагрузкой для выбора оптимальной `memory_timeout_policy` (A/B: low/base/high)
- добавить regression-checklist перед релизом
- подготовить release-пакет и финальный README
- добавить rename active profile (смена `Profile.Name` из UI без ручного JSON-edit)
- добавить экспорт/импорт всего набора профилей одним файлом (не только active profile)
- добавить drag&drop сортировку доменов внутри cover list
- прогнать runtime-валидацию cover после gateway-fix: ожидать `[Cover] gateway ...`, `Route added ...`, `-> OK`
- прогнать runtime-валидацию DNS: ожидать DNS через `10.8.0.1` внутри AWG и отсутствие unicast DNS на физический gateway
- прогнать runtime-валидацию h2-cover после Hard Mode: в логах не должно быть `malformed HTTP response` и `Rejecting ALPN "h2"`, ожидаем стабильные `-> OK` на современных доменах
- сверить `ws.txt`: после ClientHello/ALPN должны идти валидные h2 Application Data без мгновенного RST/FIN на каждом запросе
- после dual transport проверить, что для `kinopoisk.ru`/`yandex.*` больше нет `HTTP/1.x transport connection broken: malformed HTTP response`
- убрать оставшиеся legacy-упоминания `llamafile/ollama` в редких текстовых артефактах (если обнаружатся вне README/UI)

---

## [UPDATE RULE]
После каждого крупного изменения обновлять:
1. `[META]` (version/date/status)
2. `[STRUCT]` (что добавлено/убрано)
3. `[BUGS FIXED]` / `[GOTCHAS]`
4. `[STAGES]` / `[NEXT TASKS]`
