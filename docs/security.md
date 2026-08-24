# Security

See also `docs/architecture/02-security-model.md` and `docs/architecture/03-rbac-matrix.md`.

## Implemented controls

- Argon2id password hashing
- Short-lived JWT access tokens + hashed rotating refresh tokens
- Optional TOTP
- Session revoke on logout and password reset
- RBAC on every sensitive route
- Maker-checker: KYC reviewer cannot be the stored reviewer; withdrawal approver cannot be the creator
- Parameterized SQL (pgx)
- Unified error envelope without stack traces
- Security headers, CORS allowlist
- Request IDs
- Audit log append-only (table trigger forbids UPDATE/DELETE)
- API tokens stored as SHA-256, raw value shown once
- Log redaction policy: no password, token, Authorization, secrets

## Production checklist

- [ ] TLS 1.2+ at the edge
- [ ] Rotate `JWT_SECRET` and `HMAC_PEPPER`
- [ ] Disable default admin password
- [ ] IP allowlists on broker API tokens
- [ ] Enable `trading.enforce_hours` when calendars are certified
- [ ] Database role without UPDATE/DELETE on immutable tables
- [ ] Network isolation: DB not public
- [ ] WAF / rate limit at ingress in addition to app limits

## HMAC (broker API)

When `hmac_required` is set on a token:

```
signature = HMAC_SHA256(pepper || timestamp || method || path || body)
header X-Timestamp, X-Signature
```

Clock skew tolerance: 30 seconds. Broker id is taken from the token, never from the body.
