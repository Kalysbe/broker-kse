# Security Model

## Authentication

| Mechanism | Design |
|---|---|
| Password | Argon2id, unique salt, parameters in config |
| Access token | JWT, 15 minutes, `sub`, `sid`, `roles`, `broker_id`, `jti` |
| Refresh token | Opaque 256-bit, stored as SHA-256 hash, rotatable, revocable |
| 2FA | TOTP (RFC 6238), recovery codes hashed |
| Sessions | Server-side session row: IP, user agent, last seen, revoke |
| Password reset | Single-use hashed token, short TTL |
| Step-up | Re-auth required for withdrawals, role changes, token issue, limit changes |
| Broker API | Bearer token (hashed at rest) + optional HMAC (`X-Timestamp`, `X-Signature`) |

Passwords, raw tokens, Authorization headers, and private keys are never logged.

## Authorization (RBAC)

- Roles are assigned to users; permissions are assigned to roles.
- Direct user-permission grants are not used (auditability of SoD).
- Every mutating and sensitive read endpoint declares a permission code.
- SUPER_ADMIN is a break-glass role; ADMIN does **not** inherit cash movement permissions.

## Segregation of duties

| Policy | Enforcement |
|---|---|
| Maker-checker | Creator cannot approve the same `approval_request` |
| Trader vs KYC | Distinct permissions `orders.submit` vs `kyc.review` |
| Accountant vs settlement | Distinct `payments.create` vs `settlement.post` |
| Admin vs finance | Financial permissions are explicit, not implied by ADMIN |
| Auditor | Read-only audit and reports; no mutate |

Policies (`organization_policies`) are configurable: dual-control required amounts, self-approval forbidden flag, IP restriction, password policy.

## API security controls

- TLS terminated at Nginx (production)
- CORS allowlist
- Rate limiting (Redis), per user / token / IP
- IP whitelist per broker API token
- Request ID on every request (`X-Request-ID`)
- Idempotency-Key header for critical POSTs in addition to `client_order_id`
- Input validation (DTO + DB constraints)
- Parameterized SQL only (pgx)
- Secure headers: `Content-Security-Policy`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`
- CSRF: SPA uses Bearer tokens (not cookie session for API). If cookie mode is enabled, SameSite=Strict + CSRF token.

## Token lifecycle

Create → hash store → use → rotate → revoke → expire  

Raw token shown once at creation. Rotation issues a new token and revokes the previous in one transaction.

## Audit

Administrative and financial actions write to `audit_logs` (append-only for application roles). Corrections create a new audit row referencing the original. Database role for the app has no `UPDATE`/`DELETE` on `audit_logs`, `cash_ledger`, `trades`, `executions`.
