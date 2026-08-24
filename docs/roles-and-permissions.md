# User roles and permissions

Canonical matrix: `docs/architecture/03-rbac-matrix.md`.

Seeded in `backend/migrations/000008_seed.up.sql`.

Enforcement is server-side (`App.perm`). The React sidebar is not a security boundary.

Segregation of duties:

| Action | Cannot be done by |
|---|---|
| KYC approve | The same user already stored as `reviewer_id` |
| Withdrawal approve | The user in `created_by` |
| Order submit | Non-`orders.submit` roles (e.g. VIEWER, ACCOUNTANT) |
| Settlement post | Roles without `settlement.post` |
| Role assignment | Anyone except SUPER_ADMIN |
