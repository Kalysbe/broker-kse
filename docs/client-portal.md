# Client portal

Staff UI and client cabinet share the same Go services (`orders`, `risk`, `cash`, `portfolio`, `reports`, `documents`).

Do not duplicate order or ledger logic in a second backend.

Recommended later split:

- Host `/portal` as a second Vite entry or app that authenticates as `CLIENT` principal
- Grant a `CLIENT` role with: `portfolio.read`, `orders.read`, `orders.submit`, `trades.read`, `cash.read`, `documents.read`, `reports.run`, `notifications.manage`

Order submission from the portal still calls `POST /v1/orders` and the same pre-trade risk path.
