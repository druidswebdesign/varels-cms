# Frontend & Design System — varels-cms

> The authoritative spec for the UI layer: design tokens, the dashboard shell,
> shared components, dark mode and the conventions for new pages. Read this
> before touching anything under `internal/views/` or `assets/`. It describes
> the shadcn-inspired redesign started on 2026-09-29; the per-page migration is
> incremental and tracked at the end of this file.

The stack is fixed by `docs/architecture.md` §4: **Tailwind CSS v4 (standalone
CLI) + templ + HTMX 2 + Alpine.js 3**, vendored with **no npm** (ADR-0022).
This doc only defines *how* the UI is built on that stack.

---

## 1. Design tokens

Tokens live in `assets/css/input.css`. Raw values are declared on `:root` and
`.dark`, then exposed to Tailwind as utilities via `@theme inline`, so markup
uses semantic names (`bg-card`) instead of palette names (`bg-white`).

| Token | Utility examples | Purpose |
| --- | --- | --- |
| `--background` / `--foreground` | `bg-background`, `text-foreground` | Page canvas and default text |
| `--card` / `--card-foreground` | `bg-card`, `text-card-foreground` | Surfaces (cards, panels, tables) |
| `--popover` / `--popover-foreground` | `bg-popover` | Floating layers |
| `--primary` / `--primary-foreground` | `bg-primary`, `text-primary-foreground` | Primary actions |
| `--secondary` / `--secondary-foreground` | `bg-secondary` | Secondary actions / neutral pills |
| `--muted` / `--muted-foreground` | `text-muted-foreground` | De-emphasised text |
| `--accent` / `--accent-foreground` | `hover:bg-accent` | Hover / active surfaces |
| `--destructive` / `--destructive-foreground` | `bg-destructive`, `text-destructive` | Errors, danger |
| `--success`, `--warning` | — | Semantic accents (badges/flash) |
| `--border`, `--input`, `--ring` | `border-border`, `border-input`, `ring-ring` | Borders, inputs, focus rings |
| `--sidebar*` | `bg-sidebar`, `bg-sidebar-accent`, `text-sidebar-foreground` | Sidebar-specific surfaces |
| `--radius` | `rounded-sm/md/lg/xl` | Radius scale (`sm = radius-4px`, `lg = radius`) |

Rules:

- **Use semantic tokens in new markup.** `bg-card text-card-foreground
  border-border`, not `bg-white border-gray-200`.
- **Prefer spacing/layout utilities for structure**, tokens for color and radius.
- Tokens are the only place a raw color value should appear; do not hard-code
  `oklch(...)`/hex in templates.

### Tailwind v4 wiring

`assets/css/input.css` also contains:

- `@source "../../internal/views"` and `@source "../../assets/js"` — the classes
  Tailwind scans. **A class exists in the compiled CSS only if it appears
  literally in one of those trees**, so pass literal class strings (e.g.
  `@Icon("cart", "h-4 w-4")`), never build them dynamically.
- `@custom-variant dark (&:is(.dark *))` — enables `dark:` utilities driven by a
  `.dark` class on `<html>` (see §5).
- A `@layer base` that sets the default `border-color`, applies
  `bg-background text-foreground` to `<body>`, and hides `[x-cloak]` elements
  until Alpine boots.

Rebuild the stylesheet whenever classes or tokens change:

```bash
make css        # tailwindcss -i assets/css/input.css -o assets/css/app.css --minify
```
  
`assets/css/app.css` is **generated**; never edit it by hand.

---

## 2. Shell layout  

`internal/views/layouts/base.templ` renders every page:

```text
<body hx-headers='{"X-CSRF-Token":"…"}'>
  <div x-data="shell">                         # drawer + collapse + theme + lang
    @components.Sidebar(active, csrf)          # rail / drawer, sign-out at bottom
    <div class="app-content flex min-h-screen flex-col lg:pl-[var(--sidebar-w)]">
      @components.Navbar(title, active, csrf)  # sticky header + search/theme/lang
      <main class="flex-1 px-4 py-6 lg:px-8">
        <div id="flash">…</div>                # OOB flash target
        { children... }
      </main>
    </div>
  </div>
</body>
```

- `Base(title, active, csrf)` — `title` labels the current page (header + tab
  title); `active` is the sidebar key (see §4) and also the navbar title's
  `data-i18n` key; `csrf` feeds the `hx-headers` attribute and the sign-out form.
- The outer div carries the `x-data="shell"` scope shared by the sidebar, navbar
  and login page (theme, language, sidebar drawer/collapse).
- The sidebar width is `var(--sidebar-w)` (16rem, or 4rem collapsed on `lg`+);
  the content column pads left by the same property. Below `lg` it is off-canvas
  and the header's menu button toggles `sidebarOpen`.
- `<body>` sets `hx-headers` so **every** HTMX request carries the CSRF token;
  forms still include `csrf_token` hidden inputs.

---

## 3. Components

All shared UI lives in `internal/views/components/` (templ components; call with
`@components.Name(...)`).

| File | Exports | Notes |
| --- | --- | --- |
| `card.templ` | `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`, `StatCard(label, value, tone)` | `StatCard` tone: `neutral` / `success` / `danger` / `info`. Content is passed as `{ children... }`. |
| `icons.templ` | `Icon(name, class)` | Inline lucide-style stroke SVGs, no CDN. Names: `dashboard, package, layers, truck, lock, cart, users, factory, chart, trending-down, receipt, user, scroll, search, menu, sun, moon, log-out, bell, plus, chevron-right, arrow-up-right`. Unknown names render nothing. Pass size classes at the call site. |
| `button.templ` | `Button(label, kind)` | `kind`: `primary` / `secondary` / `danger` (renders `<button type="submit">`). |
| `badge.templ` | `Badge(label, tone)` | Tones: `neutral` / `success` / `warning` / `danger` / `info`. |
| `table.templ` | `Table()` | Bordered card shell; caller supplies `thead`/`tbody`. |
| `flash.templ` | `Flash(kind, message)` | Static flash block; live flashes come from `partials.FlashMessages` / `FlashOOB`. |
| `sidebar.templ`, `navbar.templ` | `Sidebar(active)`, `Navbar(title, csrf)` | Shell pieces; normally only `base.templ` calls them. |
| `modal.templ` | `Modal(id)` | Hidden-by-default container toggled by HTMX/script; retokenised (`bg-card`, `bg-black/40`). |

Composition example (the pattern new pages should follow):

```templ
@components.Card() {
  @components.CardHeader() {
    @components.CardTitle() { Products }
    @components.CardDescription() { Active catalog }
  }
  @components.CardContent() {
    @components.Table() { … }
  }
}
```

---

## 4. Sidebar navigation

`components/sidebar.templ` renders grouped links. Each link takes a **key** used
for highlighting, matched against the `active` argument passed to
`layouts.Base` by the page. Current groups/keys:

| Group | Links (key) |
| --- | --- |
| Overview | Dashboard (`dashboard`), Analytics (`analytics`) |
| Catalog | Products (`products`), Collections (`collections`) |
| Inventory | Restock (`restocks`), Stock holds (`holds`), Deadstock (`deadstock`) |
| Sales | Sales (`sales`), Customers (`customers`) |
| Finance | Expenses (`expenses`) |
| Administration | Suppliers (`suppliers`), Staff (`staff`), Audit log (`audit`) |

To add a page: add `@sidebarLink("/path", "key", "Label", "icon", active)` in the
right `@navGroup(...)` and call `@layouts.Base("Title", "key", csrf)` from the
page. Sub-pages reuse their parent key (e.g. `variant_form.templ` uses
`products`; `purchase_orders.templ` uses `suppliers`).

Mobile behaviour is Alpine-only and degrades gracefully without JS: the aside
uses a `--sidebar-w` width custom property plus `-translate-x-full lg:translate-x-0`,
`sidebarOpen` toggles the drawer, a backdrop with `x-show="sidebarOpen" x-cloak`,
and the header button sets `sidebarOpen = true`.

### 4.1 Collapsible rail (desktop)

The shell has a collapse control in the sidebar header (`panel-left-*` icon,
desktop only). Collapsed state lives in the `shell` Alpine scope and is persisted
as `localStorage.sidebarCollapsed`:

- The pre-paint `<head>` script sets `html[data-sidebar="collapsed"]`, and
  `input.css` shrinks `--sidebar-w` to `4rem` under `@media (min-width:1024px)`.
  The aside (`w-[var(--sidebar-w)]`) and content (`lg:pl-[var(--sidebar-w)]`) both
  follow, so there is no layout flash before Alpine boots.
- Labels (`data-i18n` spans) and group headings are `x-show="sidebarOpen || !collapsed"`
  — hidden on a collapsed desktop rail, visible in the mobile drawer.
- Collapsed links center their icon (`lg:justify-center`) and expose the label as
  both a `title` tooltip and an `aria-label` via `x-bind`.
- Mobile is unaffected: below `lg` the rail stays full-width as a drawer.

### 4.2 Sign out

Sign-out lives at the **bottom** of the sidebar, separated from navigation by
`mt-auto border-t border-sidebar-border` (it stays pinned regardless of menu
content or screen size). It is a `POST /logout` form carrying the CSRF token;
when the rail is collapsed it shows the icon with a tooltip/`aria-label`.

---

## 5. Dark mode

Two pieces, both required:

1. **Pre-paint** — an inline `<script>` in the `<head>` reads
   `localStorage.theme` (falling back to `prefers-color-scheme: dark`), adds
   `.dark` to `<html>` and sets `document.documentElement.style.colorScheme`
   before first paint, so there is no flash. Both `layouts/base.templ` and the
   standalone `pages/login.templ` include it.
2. **Toggle** — the `shell` Alpine component (see §6) exposes `dark` and
   `toggleTheme()`; the header button flips the class, persists
   `localStorage.theme`, and dispatches `app:theme` for chart/JS listeners.

`input.css` declares `color-scheme: light` on `:root` and `dark` on `.dark`, so
native controls (selects, scrollbars, date pickers) follow the theme too. All
theme-aware styling must use `dark:` variants or token utilities — never assume
light mode. Every page and component has been swept to tokens, so switching
themes repaints the whole app consistently (login included).

---

## 6. Preferences: theme, language, sidebar

Three persisted UI preferences live in `localStorage`, applied pre-paint by the
inline `<head>` script and managed by the `shell` Alpine component
(`assets/js/app.js`) mounted on the shell wrapper in `base.templ` (and on the
login page):

| Preference          | Key                   | Values                | Control |
| ------------------- | --------------------- | --------------------- | ------- |
| Theme               | `theme`               | `light` / `dark`      | Navbar/header sun-moon button (`toggleTheme`) |
| Language            | `lang`                | `en` / `es`           | Navbar/header `EN`/`ES` button (`toggleLang`) |
| Collapsed sidebar   | `sidebarCollapsed`    | `true` / `false`      | Sidebar header `panel-left-*` button (`toggleCollapsed`) |

Everything reads the shared `shell` scope, so the controls are keyboard-operable
real `<button>`s with `aria-label`s (`a11y.*` keys) and visible focus rings.

### 6.1 i18n (English / Spanish)

Translation is **client-side**, so switching is instant with no reload and works
for HTMX-swapped fragments too. There is no server round-trip and no `Accept-Language`
handling.

- The dictionary lives in `assets/js/app.js` (`I18N.en` / `I18N.es`). `i18n.apply(root)`
  walks the subtree and fills:
  - `data-i18n="key"` → `textContent`
  - `data-i18n-placeholder="key"` → `placeholder`
  - `data-i18n-aria="key"` → `aria-label`
  - `data-i18n-title="key"` → `title`
- Missing keys fall through to the server-rendered English text, so pages adopt
  keys incrementally: add `data-i18n` to a label and a dictionary entry, and it
  translates.
- On language change the component updates `document.documentElement.lang`,
  persists `lang`, re-runs `i18n.apply(document)` and fires `app:lang`. HTMX swaps
  re-apply to the swapped subtree in the `htmx:afterSwap` handler.
- Dynamic Alpine text uses the component method `t('key')` (e.g. sidebar tooltips).
- Default language is **English** when no preference is stored.

Server-rendered flash/toast text is not translated by this layer (it is produced
Go-side); the client-side HTMX error toasts (`msg.*`) are.

## 7. HTMX & Alpine conventions

Unchanged from `docs/architecture.md` §4.6, restated for UI work:

- HTMX does server-driven swaps; Alpine handles local state. Both are vendored
  (`assets/js/htmx.min.js`, `assets/js/alpine.min.js`) and loaded in `base.templ`.
- `assets/js/app.js` re-initialises Alpine on `htmx:afterSwap`, re-applies
  translations to the swapped subtree, toasts
  `htmx:responseError`/`htmx:sendError` into `#flash`, and registers the `shell`
  (theme/lang/sidebar) and `pos()` (POS editor) Alpine components.
- **Mutating HTMX handlers should return the refreshed fragment plus
  `partials.FlashOOB(...)`** instead of redirecting with `?err=`. This is the
  flash/OOB contract used by the variant table; the remaining `?err=` forms are
  scheduled for conversion (see §9).
- Assert on the **fragment contract** and status codes in tests, not on rendered
  classes.

---

## 8. Page anatomy

A migrated page follows this shape:

```templ
templ ProductsPage(...) {
  @layouts.Base("Products", "products", csrf) {
    <div class="space-y-1">
      <h2 class="text-2xl font-semibold tracking-tight">Products</h2>
      <p class="text-sm text-muted-foreground">…</p>
    </div>
    <div class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      @components.StatCard("Products", fmt.Sprint(n), "neutral")
    </div>
    <div class="mt-4">
      @components.Card() { … }
    </div>
  }
}
```

- Page heading is `text-2xl font-semibold tracking-tight` with a
  `text-muted-foreground` sub-line.
- KPI rows use `StatCard`; content sections use `Card`.
- Keep one `h2` per page; the shell header already renders the page `title`.

---

## 9. Migration status

The token layer and shell landed on **2026-09-29**; the dashboard was rebuilt on
it. On **2026-10-13** every page/partial was swept from legacy `gray-*`/`bg-white`
classes to semantic tokens, and the shell gained theme/language/collapse
preferences.

| Area | Status |
| --- | --- |
| Tokens + `base.templ` shell + sidebar/header | ✅ Done |
| `Card` / `StatCard` / `Icon` primitives | ✅ Done |
| `button` / `badge` / `table` / `flash` retokenised | ✅ Done |
| `dashboard.templ`, `profit_cards`, `low_stock_widget`, `flash_messages` | ✅ Done |
| `restock_form` + `partials/restock_panel` (stock panel, HTMX + flash/OOB) | ✅ Done (2026-10-13) |
| Full token sweep of all pages/partials (products, sales, analytics, admin, …) | ✅ Done (2026-10-13) |
| Theme / language / collapsed-sidebar preferences (see §6) | ✅ Done (2026-10-13) |
| Sign-out pinned to bottom of sidebar; collapsible rail + tooltips | ✅ Done (2026-10-13) |
| Restock/adjust forms → HTMX fragments; remove `?err=` redirects | ⚙️ Restock/adjust done; sale/return/variant `?err=` remain |
| Low-stock email digest | ⏳ `docs/testing.md` order 4 |

When sweeping a page, do it in one pass: swap palette classes for tokens, wrap
sections in `Card`, and keep the handler's `active` key unchanged.

---

## 10. Verification gate

Run the full gate from `docs/ai-tracking.md` §5 after any UI change:

```bash
templ generate
make css
go build ./... && go vet ./... && gofmt -l internal cmd
go test ./...
node --check assets/js/app.js
```

For anything touching the shell, HTMX or Alpine, finish with a live smoke test
against a temp DB (`APP_ENV=development DB_PATH=/tmp/…`) and confirm the pages
return 200 with no panics.

---

## 11. Related documents

| Question | Document |
| --- | --- |
| Stack, folder structure, HTMX/Alpine role | `docs/architecture.md` §4 |
| What routes/UI flows must exist | `docs/routes.md` |
| What exists now / AI tracking | `docs/ai-tracking.md` |
| What must be converted next, and test strategy | `docs/testing.md` |
| Vendored browser libs (no npm) | `adr.md` ADR-0022 |
