# Auth Model — varels-cms

> This is the authoritative auth spec. It defines how staff log in, how access is
> granted and revoked, and which alternatives were rejected and why. Format rules
> live in `adr.md` (ADR-0004).

For a private clothing-brand CMS, the best approach is an **invite-only whitelist
with Google OAuth, plus an emergency admin password** ("break-glass" login).

It is the gold standard for small-to-medium private business tools. The sections
below explain how it works in plain English and why the alternatives are worse.

---

## 1. Recommended architecture

### 1.1 Daily logins — Google OAuth via email whitelist

Instead of letting employees invent passwords, force them to sign in with their
existing Google account (personal Gmail or company Google Workspace).

- **Zero password fatigue.** Staff don't have to remember another password.
- **Built-in two-factor auth (2FA).** If staff have 2FA enabled with Google, the
  CMS gets enterprise-grade security for free, without writing any 2FA code in Go.
- **Instant offboarding.** If an employee leaves or is fired, delete their email
  from the SQLite database (or disable their company Google account). They lose
  access to the CMS, warehouse data, and profit margins immediately.

### 1.2 The gatekeeper — strict SQLite whitelist

- The login screen has one main button: **"Sign in with Google"**.
- When Google returns the user's email, Go checks SQLite:
  - **On the `approved_users` table?**
    - **YES** → create a session cookie and let them in.
    - **NO** → show: _"Access Denied: Your email is not authorized to view this
      system."_

### 1.3 The safety valve — one emergency "break-glass" local admin

Always create one hidden, local admin account with a traditional email and a very
strong password stored in the database.

- **Why:** If Google's login service goes down, or you accidentally revoke your
  Google Cloud API credentials, the business owner can still log in at the manual
  `/admin-login` link to restock clothes and view data.

---

## 2. Why other approaches are worse

| Approach                         | Why it's NOT recommended for this app                                                                                                     |
| -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Public email/password signups    | Terrible for private apps. Strangers can create accounts, forcing you to monitor and delete spam users.                                   |
| Open Google login (no whitelist) | Anyone with a `@gmail.com` address could log in and see wholesale prices and inventory.                                                   |
| Pure email/password (no Google)  | Requires building forgot-password, reset emails, SMTP setup, and password rules — a massive waste of time for an internal tool.           |
| Paid auth SaaS (Auth0, Clerk)    | Overkill. They charge monthly fees once you need custom domains or more users, and add external complexity to a simple Go + SQLite setup. |

---

## 3. Seeding the very first user

A common question: _"If it's invite-only, how do I get the first user in?"_

1. On first run, a small Go startup script checks whether the database is empty.
2. If empty, it creates the brand owner email (e.g.
   `owner@clothingbrand.com`) as the `admin` owner.
3. You log in with Google using that email.
4. From the dashboard, use the **"Invite Staff"** page to enter employees' emails
   and assign roles (`admin` or `staff`). Clients and providers do **not** get
   login accounts (ADR-0018).

---

## 4. Security hardening

What the server enforces at runtime, in code:

- **Whitelist on both login paths** — Google OAuth and the break-glass login both
  require `is_active = 1` and a lower-cased email match, so disabling a row
  revokes access on the next request.
- **Server-side sessions** — session data lives in the `sessions` table keyed by
  a random token (not a signed cookie). Expired sessions are rejected and purged
  by an hourly cleanup job.
- **bcrypt + no user enumeration** — local passwords are bcrypt-hashed; a dummy
  bcrypt compare equalises timing when the account does not exist.
- **CSRF** — a double-submit cookie is checked on every POST/PUT/PATCH/DELETE
  with a constant-time compare.
- **Session-fixation protection** — the session token is rotated on login.
- **Rate limiting + audit** — the break-glass login allows 5 failures per 15
  minutes per IP and records every login in `audit_logs`.
- **Security headers** — every response carries a Content-Security-Policy
  (same-origin, allowing only the inline theme script and the ApexCharts CDN),
  `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: no-referrer`, and a restrictive `Permissions-Policy`.

### Required for a private production deployment

The server refuses to start in production (`APP_ENV != development`) unless:

- `APP_ENV=production`.
- `SESSION_SECRET` is at least 32 bytes (e.g. `openssl rand -hex 32`). A missing
  or short secret is fatal.
- `ADMIN_PASSWORD` (break-glass) is at least 12 characters. A weak password is
  fatal **before** any database write.

Additionally, HTTPS must terminate in front of the app: the binary serves plain
HTTP, so put nginx/Caddy/Cloudflare in front. Set `Strict-Transport-Security`
at that proxy.

### Reverse proxy note

`ClientIP` deliberately ignores `X-Forwarded-For` because no proxy is configured.
If you later deploy behind a reverse proxy, the rate limiter and audit-log IPs
will all see the proxy's address until trusted-proxy support is added — don't
rely on per-IP limits or IP auditing until then.
