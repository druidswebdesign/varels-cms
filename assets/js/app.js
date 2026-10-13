// Frontend glue for HTMX + Alpine.js + ApexCharts.
//
// HTMX swaps DOM fragments server-side; Alpine v3 initialises the swapped tree
// (its MutationObserver would also do this, but we init synchronously inside
// htmx:afterSwap so new Alpine components are live before the next paint).
// Failed HTMX requests are surfaced as toasts in the #flash region.
//
// The shell also owns three persisted UI preferences (localStorage):
//   - theme:              "light" | "dark"    (applied pre-paint in the layout)
//   - lang:               "en" | "es"          (client-side i18n, no reload)
//   - sidebarCollapsed:   "true" | "false"     (desktop icon-only rail)

// --- i18n ------------------------------------------------------------------
// English/Spanish dictionary. Missing keys fall through to the server-rendered
// English text, so pages can adopt `data-i18n` incrementally.
const I18N = {
  en: {
    "nav.overview": "Overview",
    "nav.catalog": "Catalog",
    "nav.inventory": "Inventory",
    "nav.sales": "Sales",
    "nav.finance": "Finance",
    "nav.administration": "Administration",
    "nav.dashboard": "Dashboard",
    "nav.analytics": "Analytics",
    "nav.products": "Products",
    "nav.collections": "Collections",
    "nav.restocks": "Restock",
    "nav.holds": "Stock holds",
    "nav.deadstock": "Deadstock",
    "nav.customers": "Customers",
    "nav.expenses": "Expenses",
    "nav.suppliers": "Suppliers",
    "nav.staff": "Staff",
    "nav.audit": "Audit log",
    "nav.searchPlaceholder": "Search products…",
    "nav.signOut": "Sign out",
    "a11y.openNav": "Open navigation",
    "a11y.toggleTheme": "Toggle theme",
    "a11y.toggleLang": "Switch language",
    "a11y.collapseSidebar": "Collapse sidebar",
    "a11y.expandSidebar": "Expand sidebar",
    "page.dashboard": "Dashboard",
    "page.analytics": "Analytics",
    "page.products": "Products",
    "page.collections": "Collections",
    "page.restocks": "Restock",
    "page.holds": "Stock holds",
    "page.deadstock": "Deadstock",
    "page.sales": "Sales",
    "page.customers": "Customers",
    "page.expenses": "Expenses",
    "page.suppliers": "Suppliers",
    "page.staff": "Staff",
    "page.audit": "Audit log",
    "msg.requestFailed": "Request failed",
    "msg.networkError": "Network error — check your connection.",
    "login.subtitle": "Sign in to your account",
    "login.email": "Email",
    "login.password": "Password",
    "login.signIn": "Sign in",
  },
  es: {
    "nav.overview": "Resumen",
    "nav.catalog": "Catálogo",
    "nav.inventory": "Inventario",
    "nav.sales": "Ventas",
    "nav.finance": "Finanzas",
    "nav.administration": "Administración",
    "nav.dashboard": "Panel",
    "nav.analytics": "Analítica",
    "nav.products": "Productos",
    "nav.collections": "Colecciones",
    "nav.restocks": "Reposición",
    "nav.holds": "Reservas",
    "nav.deadstock": "Stock inmovilizado",
    "nav.customers": "Clientes",
    "nav.expenses": "Gastos",
    "nav.suppliers": "Proveedores",
    "nav.staff": "Personal",
    "nav.audit": "Auditoría",
    "nav.searchPlaceholder": "Buscar productos…",
    "nav.signOut": "Cerrar sesión",
    "a11y.openNav": "Abrir navegación",
    "a11y.toggleTheme": "Cambiar tema",
    "a11y.toggleLang": "Cambiar idioma",
    "a11y.collapseSidebar": "Contraer barra lateral",
    "a11y.expandSidebar": "Expandir barra lateral",
    "page.dashboard": "Panel",
    "page.analytics": "Analítica",
    "page.products": "Productos",
    "page.collections": "Colecciones",
    "page.restocks": "Reposición",
    "page.holds": "Reservas",
    "page.deadstock": "Stock inmovilizado",
    "page.sales": "Ventas",
    "page.customers": "Clientes",
    "page.expenses": "Gastos",
    "page.suppliers": "Proveedores",
    "page.staff": "Personal",
    "page.audit": "Auditoría",
    "msg.requestFailed": "La solicitud falló",
    "msg.networkError": "Error de red: revisá tu conexión.",
    "login.subtitle": "Iniciá sesión en tu cuenta",
    "login.email": "Correo electrónico",
    "login.password": "Contraseña",
    "login.signIn": "Iniciar sesión",
  },
};

const i18n = {
  lang: "en",
  t(key, lang = this.lang) {
    const dict = I18N[lang] || I18N.en;
    return dict[key] != null ? dict[key] : I18N.en[key];
  },
  // Apply translations to a subtree (document by default). Missing keys leave
  // the existing text untouched.
  apply(root, lang = this.lang) {
    const el = root || document;
    el.querySelectorAll("[data-i18n]").forEach((node) => {
      const value = this.t(node.getAttribute("data-i18n"), lang);
      if (value != null) node.textContent = value;
    });
    el.querySelectorAll("[data-i18n-placeholder]").forEach((node) => {
      const value = this.t(node.getAttribute("data-i18n-placeholder"), lang);
      if (value != null) node.setAttribute("placeholder", value);
    });
    el.querySelectorAll("[data-i18n-aria]").forEach((node) => {
      const value = this.t(node.getAttribute("data-i18n-aria"), lang);
      if (value != null) node.setAttribute("aria-label", value);
    });
    el.querySelectorAll("[data-i18n-title]").forEach((node) => {
      const value = this.t(node.getAttribute("data-i18n-title"), lang);
      if (value != null) node.setAttribute("title", value);
    });
  },
};

// Set the active language before Alpine boots, then translate the document.
(function () {
  const stored = (() => {
    try {
      return localStorage.getItem("lang");
    } catch (e) {
      return null;
    }
  })();
  i18n.lang = stored === "es" ? "es" : "en";
  document.documentElement.lang = i18n.lang;
})();

function applyStoredLanguage() {
  i18n.apply(document);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", applyStoredLanguage);
} else {
  applyStoredLanguage();
}

// --- HTMX glue -------------------------------------------------------------
// Re-initialise Alpine on swapped content, re-apply translations, and let chart
// partials re-render.
document.addEventListener("htmx:afterSwap", (event) => {
  const target = event.detail && event.detail.target;
  if (window.Alpine && target && !target._x_dataStack) {
    window.Alpine.initTree(target);
  }
  if (target) i18n.apply(target);
  window.dispatchEvent(new CustomEvent("app:swap", { detail: { target } }));
});

// Toast a failed HTMX request (e.g. 4xx/5xx that did not swap).
document.addEventListener("htmx:responseError", (event) => {
  const status = event.detail && event.detail.xhr ? event.detail.xhr.status : "?";
  showFlash(`${i18n.t("msg.requestFailed")} (${status})`, "error");
});

// Toast network / connection failures.
document.addEventListener("htmx:sendError", () => {
  showFlash(i18n.t("msg.networkError"), "error");
});

// showFlash prepends a dismissible-by-timeout toast into the #flash slot.
function showFlash(message, tone) {
  const host = document.getElementById("flash");
  if (!host) return;
  const el = document.createElement("div");
  el.setAttribute("role", "alert");
  el.className =
    tone === "error"
      ? "mb-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
      : "mb-2 rounded-md border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700 dark:text-emerald-300";
  el.textContent = message;
  host.prepend(el);
  setTimeout(() => el.remove(), 6000);
}

// Shared Alpine store: `$store.app.flash('message')` from any component.
document.addEventListener("alpine:init", () => {
  window.Alpine.store("app", {
    flash(message, tone = "success") {
      showFlash(message, tone);
    },
  });

  // Shell: sidebar drawer + collapse, theme and language. Persisted in
  // localStorage; the pre-paint script already applied theme/lang/collapse, so
  // initial state is read back from the document to avoid a flash.
  window.Alpine.data("shell", () => ({
    sidebarOpen: false,
    collapsed: document.documentElement.dataset.sidebar === "collapsed",
    lang: i18n.lang,
    dark: document.documentElement.classList.contains("dark"),

    init() {
      i18n.lang = this.lang;
      i18n.apply(document);
    },

    t(key) {
      return i18n.t(key, this.lang);
    },

    toggleTheme() {
      this.dark = !this.dark;
      document.documentElement.classList.toggle("dark", this.dark);
      document.documentElement.style.colorScheme = this.dark ? "dark" : "light";
      try {
        localStorage.setItem("theme", this.dark ? "dark" : "light");
      } catch (e) {}
      window.dispatchEvent(new CustomEvent("app:theme", { detail: { dark: this.dark } }));
    },

    setLang(lang) {
      this.lang = lang === "es" ? "es" : "en";
      i18n.lang = this.lang;
      document.documentElement.lang = this.lang;
      try {
        localStorage.setItem("lang", this.lang);
      } catch (e) {}
      i18n.apply(document);
      window.dispatchEvent(new CustomEvent("app:lang", { detail: { lang: this.lang } }));
    },

    toggleLang() {
      this.setLang(this.lang === "es" ? "en" : "es");
    },

    toggleCollapsed() {
      this.collapsed = !this.collapsed;
      document.documentElement.dataset.sidebar = this.collapsed ? "collapsed" : "expanded";
      try {
        localStorage.setItem("sidebarCollapsed", String(this.collapsed));
      } catch (e) {}
    },
  }));

  // POS line editor (pages/sale_form.templ). Keeps repeated field names so the
  // handler still receives variant_id/quantity/unit_price/line_discount arrays.
  window.Alpine.data("pos", () => ({
    lines: [],
    orderDiscount: 0,
    shipping: 0,
    seq: 0,
    init() {
      for (let i = 0; i < 3; i++) this.addLine();
    },
    addLine() {
      this.lines.push({
        key: ++this.seq,
        variant_id: "",
        quantity: 1,
        unit_price: null,
        line_discount: null,
      });
    },
    removeLine(index) {
      this.lines.splice(index, 1);
    },
    onVariant(event, index) {
      const opt = event.target.selectedOptions[0];
      const retail = opt ? parseInt(opt.dataset.retail || "0", 10) : 0;
      this.lines[index].unit_price = retail > 0 ? retail / 100 : null;
      this.recalc();
    },
    lineSubtotal(line) {
      const qty = Number(line.quantity) || 0;
      const price = Number(line.unit_price) || 0;
      const disc = Number(line.line_discount) || 0;
      const s = qty * price - disc;
      return s > 0 ? s : 0;
    },
    subtotal() {
      return this.lines.reduce((sum, l) => sum + this.lineSubtotal(l), 0);
    },
    total() {
      const t =
        this.subtotal() -
        (Number(this.orderDiscount) || 0) +
        (Number(this.shipping) || 0);
      return t > 0 ? t : 0;
    },
    money(n) {
      return (
        "$" +
        (Number(n) || 0).toLocaleString("es-AR", {
          minimumFractionDigits: 2,
          maximumFractionDigits: 2,
        })
      );
    },
    recalc() {},
  }));
});
