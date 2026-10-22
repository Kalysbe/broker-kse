# Руководство для брокеров — этап 1

Шлюз принимает команды по **HTTPS REST**, а события реального времени передаёт через **gRPC bidirectional streaming + Protobuf**.

```text
Ваша система  →  HTTPS + Bearer-токен  →  Gateway  →  FIX  →  QUIK
```

Интерактивная схема: `{BASE_URL}/swagger`  
OpenAPI: `{BASE_URL}/openapi.yaml` и `api/broker-v1.yaml`  
Пример базового URL: `https://gateway.example.com` (локально `http://localhost:18080`)

---

## 1. Что выдаёт оператор

| Что | Зачем |
|-----|--------|
| Базовый URL шлюза | Все запросы идут сюда |
| API-токен `sk_test_…` или `sk_live_…` | Авторизация. Показывается **один раз** |
| Код фирмы `quik_firm` | Привязан к токену, уходит в QUIK как FIX tag 5018 |
| Торговый счёт `account_id` | Например `1-3101-006` → FIX tag 1 |
| Код клиента `client_code` | Например `006` → FIX tag 109 |
| Код класса `board` | Например `LISTEDSTOCKK` → FIX tag 100 |
| Список инструментов | Или берите сами: `GET /api/v1/instruments` |

Токен храните как секрет. В логах оставляйте только префикс (`sk_live_AbCd…`). При утечке оператор отзывает токен и выдаёт новый.

Локальный seed (только development): `GATEWAY_DEV_TOKEN` (по умолчанию `sk_test_devStage1TokenDoNotUseInProd`).

---

## 2. Авторизация

Каждый запрос к `/api/v1/…`:

```http
Authorization: Bearer sk_live_…
Content-Type: application/json
```

- `broker_id` берётся из токена, в теле заявки его нет.
- Если включён IP-whitelist, запросы с других адресов вернут `403` (`ip not allowed`).
- Права токена по умолчанию: `orders:create`, `orders:read`, `orders:cancel`, `trades:read`, `positions:read`.
- HMAC-подпись на этапе 1 **не обязательна**. Если оператор включит `HMAC_REQUIRED`, см. приложение ниже.

Проверка связи:

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$BASE_URL/api/v1/firm"
```

Ожидается:

```json
{ "quik_firm": "MC0031400000", "firm_name": "ТОО Example" }
```

Если `quik_firm` пустой — заявки создавать нельзя.

---

## 3. Методы этапа 1

| Метод | Путь | Право | Назначение |
|-------|------|-------|------------|
| `GET` | `/api/v1/firm` | — | Код фирмы текущего токена |
| `GET` | `/api/v1/instruments` | `orders:read` | Справочник инструментов |
| `GET` | `/api/v1/accounts` | `orders:read` | Счета брокера |
| `POST` | `/api/v1/orders` | `orders:create` | Создать заявку |
| `GET` | `/api/v1/orders` | `orders:read` | Список заявок (до 100) |
| `GET` | `/api/v1/orders/{id}` | `orders:read` | Заявка + события + сделки |
| `POST` | `/api/v1/orders/{id}/cancel` | `orders:cancel` | Снять заявку |
| `GET` | `/api/v1/trades` | `trades:read` | Сделки (до 100) |
| `GET` | `/api/v1/positions` | `positions:read` | Последний снимок позиций |

`{id}` — UUID заявки из ответа шлюза (`order.id`), не `client_order_id`.

Замены заявки (`replace`) на этапе 1 нет. Нужно изменить цену или количество — снимите заявку и создайте новую с **другим** `client_order_id`.

---

## 4. Создать заявку

`POST /api/v1/orders`

Обязательные поля: `account_id`, `client_code`, `client_order_id`, `symbol`, `board`, `side`, `order_type`, `quantity`.  
`price` обязателен для `LIMIT`. `time_in_force` по умолчанию `DAY`.

Не передавайте `broker_id`, `quik_firm`, валюту и неизвестные поля: JSON с лишним ключом вернёт `400`.

- `201` — заявка создана (`idempotent: false`), статус обычно `QUEUED`
- `200` — повтор с тем же `client_order_id` (`idempotent: true`)

`201` означает «принято шлюзом», а не «стоит в стакане». Дальше опрашивайте `GET /api/v1/orders/{id}`.

---

## 5. Идемпотентность

Ключ: пара `(broker_id, client_order_id)`.

1. Генерируйте `client_order_id` у себя **до** отправки.
2. При обрыве сети повторите тот же `POST` с тем же `client_order_id`.
3. Шлюз **не сравнивает** тело повторного запроса с первым.
4. Рекомендуемый формат: `ORD-{yyyyMMdd}-{seq}`.

---

## 6. Статусы и опрос

`QUEUED` → `ROUTING` → `SENT` → `ACKNOWLEDGED` → `PARTIALLY_FILLED` / `FILLED`

Также: `CANCEL_PENDING`, `CANCELLED`, `REJECTED`, `EXPIRED`, `FAILED`.

Конечные: `FILLED`, `CANCELLED`, `REJECTED`, `EXPIRED`, `FAILED`.

На этапе 1 **пуша нет**. Рекомендация: первые 5 секунд — каждые 200–500 мс, далее 1–2 с.

---

## 7. Снять заявку

`POST /api/v1/orders/{id}/cancel` — тело не нужно.

Снять можно из `ACKNOWLEDGED` и `PARTIALLY_FILLED`. Из `QUEUED` / `SENT` / конечных статусов вернётся `400`.

`200` — запрос принят. Дождитесь `CANCELLED` или `FILLED`.

---

## 8. Ошибки

```json
{ "error": "текст", "details": {} }
```

| HTTP | Когда |
|------|--------|
| `400` | Неверный JSON, валидация, нет инструмента, нельзя снять |
| `401` | Нет токена, неверный или просроченный токен, плохая HMAC-подпись |
| `403` | Нет права на метод или IP не в whitelist |
| `404` | Заявка не найдена или чужая |
| `429` | Лимит запросов или заявок (100 rps / 20 create rps) |
| `503` | Шлюз не готов (`GET /ready`) |

---

## 9. Чеклист первого дня

1. `GET /health` и `GET /ready`
2. `GET /api/v1/firm`
3. `GET /api/v1/instruments`
4. `POST /api/v1/orders` → `201`
5. Повтор того же `POST` → `200`, `idempotent: true`
6. `GET /api/v1/orders/{id}` до `ACKNOWLEDGED` / `FILLED` / `REJECTED`
7. Для лимитки: `POST …/cancel` из `ACKNOWLEDGED`, опрос до `CANCELLED`

---

## 10. Что не входит в этап 1

- webhook
- Входящий FIX от брокера
- Replace / amend
- Рыночные данные в брокерском API
- HMAC по умолчанию

---

## Приложение: HMAC

Нужно, если оператор включил `HMAC_REQUIRED` или вы сами шлёте `X-Signature`.

- `X-Timestamp` — Unix time в секундах, окно ±5 минут
- `X-Nonce` — уникальная строка на запрос
- `X-Signature` — hex(HMAC-SHA256(token, payload))

```text
{timestamp}\n{nonce}\n{METHOD}\n{path}\n{body}
```

Пример path: `/api/v1/orders`. Секрет — **сам API-токен**. Для `GET` тело пустое.
