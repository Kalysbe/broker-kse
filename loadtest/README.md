# Нагрузочное тестирование заявок

Генератор проверяет весь цикл заявки: отправку, HTTP-подтверждение и последующий
поиск по биржевому `order.id`. BUY и SELL чередуются, каждому запросу назначается
уникальный `client_order_id`. JSON и CSV сохраняются в `loadtest/reports/`.

По умолчанию выполняется только безопасный dry-run:

```powershell
docker compose --profile loadtest run --rm order-load
```

Реальный тест нашего API (который маршрутизирует заявки в тестовый шлюз биржи):

```powershell
$env:LOAD_TOKEN="sk_test_..."
$env:LOAD_RATE="1000"
$env:LOAD_DURATION="60s"
$env:LOAD_WORKERS="500"
$env:LOAD_EXECUTE="true"
docker compose --profile loadtest run --rm order-load
```

Прямой тест тестового API биржи:

```powershell
$env:LOAD_BASE_URL="http://host.docker.internal:8080"
$env:LOAD_ACCOUNT="1-3101-2040"
$env:LOAD_CLIENT="Kalys"
$env:LOAD_SYMBOL="GBA02280810"
$env:LOAD_BOARD="LIST_GBA"
$env:LOAD_PRICE="90"
docker compose --profile loadtest run --rm order-load
```

`sk_live_` заблокирован. Для его использования нужно отдельно передать
`LOAD_ALLOW_LIVE=true`; делать это можно только в согласованное биржей окно.

У тестового токена нашего API стандартный лимит — 20 заявок/с. Для проверки
большего RPS создайте отдельный тестовый токен и заранее установите ему
`rate_limit_rps` и `order_rate_limit_rps` не ниже `LOAD_RATE`. Ответы `429`
считаются провалом теста, а не потерянными заявками.

Итог содержит количество принятых, отклонённых, транспортных ошибок, потерянных
после подтверждения, дублей и задержки p50/p95/p99. Ненулевые потери, дубли или
транспортные ошибки, HTTP-отклонения или заявки, не дошедшие до биржевого
статуса, завершают процесс с кодом 1.
