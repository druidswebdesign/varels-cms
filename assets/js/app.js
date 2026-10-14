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

// --- Global phrase table ---------------------------------------------------
// The `I18N` map above is keyed by explicit `data-i18n` keys. Pages, tables,
// forms and options across the app render plain English with no key, so this
// table translates the *source text itself* (exact, case-sensitive match on the
// trimmed text/attribute value). It keeps the whole UI bilingual without adding
// markup to every node, and works for HTMX-swapped fragments too. Original text
// is stashed per node so toggling back to English restores it exactly.
//
// Keys are the English strings as rendered; values are Spanish.
const PHRASES = {
  // Shared labels
  "Name": "Nombre",
  "Display name": "Nombre visible",
  "Note": "Nota",
  "Notes": "Notas",
  "Status": "Estado",
  "Location": "Ubicación",
  "Variant": "Variante",
  "Variants": "Variantes",
  "Variant ID": "ID de variante",
  "Location ID": "ID de ubicación",
  "Email": "Correo",
  "Qty": "Cant.",
  "Quantity": "Cantidad",
  "Quantity (±)": "Cantidad (±)",
  "When": "Fecha",
  "Back": "Volver",
  "Edit": "Editar",
  "Filter": "Filtrar",
  "Apply": "Aplicar",
  "Search": "Buscar",
  "Clear": "Limpiar",
  "Clear filters": "Limpiar filtros",
  "Save": "Guardar",
  "Save changes": "Guardar cambios",
  "Cancel": "Cancelar",
  "Delete": "Eliminar",
  "Archive": "Archivar",
  "Restore": "Restaurar",
  "Release": "Liberar",
  "Reset": "Restablecer",
  "Disable": "Deshabilitar",
  "Enable": "Habilitar",
  "Invite": "Invitar",
  "Create": "Crear",
  "Create one": "Crear una",
  "Set status": "Establecer estado",
  "SKU": "SKU",
  "Category": "Categoría",
  "Color": "Color",
  "Date": "Fecha",
  "Phone": "Teléfono",
  "Reason": "Motivo",
  "Size": "Talle",
  "State": "Estado",
  "Total": "Total",
  "Totals": "Totales",
  "Total:": "Total:",
  "Subtotal": "Subtotal",
  "Threshold": "Umbral",
  "Type": "Tipo",
  "URL": "URL",
  "Unit": "Unidad",
  "Unit cost": "Costo unitario",
  "Unit cost (ARS)": "Costo unitario (ARS)",
  "Unit price (ARS)": "Precio unitario (ARS)",
  "Units": "Unidades",
  "User": "Usuario",
  "User ID": "ID de usuario",
  "Action": "Acción",
  "Entity": "Entidad",
  "Entity type": "Tipo de entidad",
  "From": "Desde",
  "To": "Hasta",
  "Month": "Mes",
  "Year": "Año",
  "Season": "Temporada",
  "Launch": "Lanzamiento",
  "Launch date": "Fecha de lanzamiento",
  "Role": "Rol",
  "Provider": "Proveedor de acceso",
  "Last login": "Último acceso",
  "Password": "Contraseña",
  "new password": "nueva contraseña",
  "Contact": "Contacto",
  "contact": "contacto",
  "Instagram": "Instagram",
  "Lead time": "Plazo de entrega",
  "Lead time (days)": "Plazo de entrega (días)",
  "lead days": "días de plazo",
  "terms": "términos",
  "Amount": "Importe",
  "Amount (ARS)": "Importe (ARS)",
  "Cost": "Costo",
  "Cost (ARS)": "Costo (ARS)",
  "Retail": "Minorista",
  "Retail (ARS)": "Minorista (ARS)",
  "Wholesale (ARS)": "Mayorista (ARS)",
  "Margin": "Margen",
  "Discount": "Descuento",
  "Discount (ARS)": "Descuento (ARS)",
  "Shipping": "Envío",
  "Shipping (ARS)": "Envío (ARS)",
  "Low-stock threshold": "Umbral de stock bajo",
  "Barcode": "Código de barras",
  "Description": "Descripción",
  "Details": "Detalles",
  "Media": "Medios",
  "Alt text": "Texto alternativo",
  "Source": "Origen",
  "Ref": "Ref.",
  "Ref type": "Tipo de ref.",
  "Reference": "Referencia",
  "Order": "Pedido",
  "Orders": "Pedidos",
  "Order discount (ARS)": "Descuento del pedido (ARS)",
  "Item": "Artículo",
  "Items": "Artículos",
  "Channel": "Canal",
  "Payment": "Pago",
  "Payments": "Pagos",
  "Method": "Método",
  "Receipt": "Recibo",
  "Returns": "Devoluciones",
  "Return qty": "Cant. a devolver",
  "Returnable": "Retornable",
  "Returned": "Devuelto",
  "Delta": "Variación",
  "Condition": "Condición",
  "Resolution": "Resolución",
  "Vendor": "Proveedor",
  "Supplier": "Proveedor",
  "Suppliers": "Proveedores",
  "Purchase orders": "Órdenes de compra",
  "Rate": "Tasa",
  "Stock movements": "Movimientos de stock",
  "Store credit": "Saldo a favor",
  "Store credit:": "Saldo a favor:",
  "Store credit balance": "Saldo a favor disponible",
  "Store credit ledger": "Libro de saldo a favor",
  "Total this view:": "Total de esta vista:",
  "Trapped cash": "Capital inmovilizado",
  "Trapped cash:": "Capital inmovilizado:",
  "On hand": "Disponible",
  "Stock": "Stock",
  "Set": "Fijar",
  "Point of sale": "Punto de venta",
  "New sale": "Nueva venta",
  "New sale (POS)": "Nueva venta (POS)",
  "New product": "Nuevo producto",
  "New PO": "Nueva OC",
  "Edit product": "Editar producto",
  "Create product": "Crear producto",
  "Edit variant": "Editar variante",
  "Add variant": "Agregar variante",
  "Save variant": "Guardar variante",
  "Save collections": "Guardar colecciones",
  "Add category": "Agregar categoría",
  "Add customer": "Agregar cliente",
  "Add expense": "Agregar gasto",
  "Add line": "Agregar línea",
  "Add media": "Agregar medio",
  "Add payment": "Agregar pago",
  "Add supplier": "Agregar proveedor",
  "Adjust stock": "Ajustar stock",
  "Adjust": "Ajustar",
  "Receive stock": "Recibir stock",
  "Receive": "Recibir",
  "Receive location": "Ubicación de recepción",
  "Received": "Recibido",
  "Ordered": "Solicitado",
  "Expected": "Previsto",
  "Completed": "Completado",
  "Complete sale": "Completar venta",
  "Process return": "Procesar devolución",
  "Create hold": "Crear reserva",
  "All": "Todos",
  "All locations": "Todas las ubicaciones",
  "All statuses": "Todos los estados",
  "Show archived": "Mostrar archivados",
  "Main navigation": "Navegación principal",
  "varels-cms home": "inicio de varels-cms",
  // Page headings / areas
  "Dashboard": "Panel",
  "Analytics": "Analítica",
  "Products": "Productos",
  "Collections": "Colecciones",
  "Restock": "Reponer",
  "Restocks": "Reposiciones",
  "Restocks only": "Solo reposiciones",
  "Full ledger": "Libro completo",
  "Deadstock": "Stock inmovilizado",
  "Sales": "Ventas",
  "Customers": "Clientes",
  "Expenses": "Gastos",
  "Expense categories": "Categorías de gastos",
  "Staff": "Personal",
  "Audit log": "Auditoría",
  "Stock holds": "Reservas de stock",
  "Best sellers": "Más vendidos",
  "Size curve": "Curva de talles",
  "Revenue by channel": "Ingresos por canal",
  "Current stock": "Stock actual",
  // Dashboard
  "Inventory and profit at a glance.": "Inventario y ganancias de un vistazo.",
  "Active variants": "Variantes activas",
  "Quick actions": "Acciones rápidas",
  "Low stock": "Stock bajo",
  "Shortcuts": "Atajos",
  "Jump to common tasks": "Ir a tareas frecuentes",
  "Loading low stock…": "Cargando stock bajo…",
  "Revenue": "Ingresos",
  "Gross profit": "Ganancia bruta",
  "Net profit": "Ganancia neta",
  "Gross margin": "Margen bruto",
  "COGS": "Costo de ventas",
  "OpEx": "Gastos operativos",
  "Inventory cost": "Costo de inventario",
  "Potential retail": "Minorista potencial",
  "Non-sellable cost": "Costo no vendible",
  // Product / variant
  "Add at least one line with a positive quantity.": "Agregá al menos una línea con cantidad positiva.",
  "Store credit requires a customer on the sale.": "El saldo a favor requiere un cliente en la venta.",
  "The customer does not have enough store credit.": "El cliente no tiene saldo a favor suficiente.",
  "Not enough stock for one of the lines. Nothing was saved.": "No hay stock suficiente para una de las líneas. No se guardó nada.",
  "Invalid sale. Check channel, location and payment method.": "Venta inválida. Revisá canal, ubicación y método de pago.",
  "No collections defined.": "No hay colecciones definidas.",
  "No media linked.": "No hay medios vinculados.",
  "No variants yet.": "Todavía no hay variantes.",
  // Empty states
  "No audit entries found.": "No se encontraron registros de auditoría.",
  "No categories yet.": "Todavía no hay categorías.",
  "No collections yet.": "Todavía no hay colecciones.",
  "No customers found.": "No se encontraron clientes.",
  "No expenses recorded.": "No hay gastos registrados.",
  "No holds found.": "No se encontraron reservas.",
  "No lines yet. Add one below.": "Todavía no hay líneas. Agregá una abajo.",
  "No movements found.": "No se encontraron movimientos.",
  "No orders yet.": "Todavía no hay pedidos.",
  "No payments recorded.": "No hay pagos registrados.",
  "No products found.": "No se encontraron productos.",
  "No purchase orders yet.": "Todavía no hay órdenes de compra.",
  "No restocks yet.": "Todavía no hay reposiciones.",
  "No sales found.": "No se encontraron ventas.",
  "No sales in this period.": "No hay ventas en este período.",
  "No stock recorded yet.": "Todavía no hay stock registrado.",
  "No suppliers yet.": "Todavía no hay proveedores.",
  "Nothing stale. Good.": "Nada inmovilizado. Bien.",
  "Everything is above threshold.": "Todo está por encima del umbral.",
  // Mixed fragments next to dynamic values
  "· receive at": "· recibir en",
  "· expected": "· previsto",
  "· no sales in": "· sin ventas en",
  "Return ·": "Devolución ·",
  "days": "días",
  "units": "unidades",
  "Sign in · varels-cms": "Iniciar sesión · varels-cms",
  // Select options / placeholders
  "— anonymous —": "— anónimo —",
  "— select —": "— seleccionar —",
  "…or new customer name": "…o nombre de nuevo cliente",
  "new customer phone": "teléfono de nuevo cliente",
  "order": "pedido",
  "name": "nombre",
  "phone": "teléfono",
  "email": "correo",
  "notes": "notas",
  "Cash": "Efectivo",
  "Card": "Tarjeta",
  "MercadoPago": "MercadoPago",
  "Transfer": "Transferencia",
  "Sellable": "Vendible",
  "Damaged": "Dañado",
  "Sample": "Muestra",
  "Gift": "Regalo",
  "Personal": "Personal",
  "Refund": "Reembolso",
  "Exchange": "Cambio",
  // Status / enum values
  "active": "activa",
  "inactive": "inactiva",
  "released": "liberada",
  "consumed": "consumida",
  "expired": "vencida",
  "draft": "borrador",
  "ordered": "solicitado",
  "cancelled": "cancelada",
  "completed": "completada",
  "complete": "completo",
  "returned": "devuelta",
  "partially_returned": "devuelta parcialmente",
  "hold": "reserva",
  "return": "devolución",
  "adjustment": "ajuste",
  "po_receipt": "recepción de OC",
  "transfer": "transferencia",
  "manual": "manual",
  "cart": "carrito",
  "preorder": "preventa",
  "cash": "efectivo",
  "card": "tarjeta",
  "mercadopago": "mercadopago",
  "store_credit": "saldo a favor",
  "sellable": "vendible",
  "damaged": "dañado",
  "sample": "muestra",
  "gift": "regalo",
  "personal": "personal",
  "image": "imagen",
  "size_chart": "tabla de talles",
  "video": "video",
  "system": "sistema",
  "staff": "personal",
  "admin": "administrador",
  "primary": "primario",
  "secondary": "secundario",
  "drop": "Drop",
  "season": "temporada",
  "essentials": "Esenciales",
  // Badge labels
  "Active": "Activo",
  "Archived": "Archivado",
  "Disabled": "Deshabilitado",
  "Inactive": "Inactivo",
  "Primary": "Principal",
  // Placeholders in search fields
  "Search name or slug": "Buscar nombre o slug",
  "Search name, phone or email": "Buscar nombre, teléfono o correo",
  "Search products…": "Buscar productos…",
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
    applyPhrases(el, lang);
  },
};

// Translate a single string by exact source-text match (see PHRASES). Returns
// the translated string, or null when the text has no entry.
function phraseFor(text, lang) {
  if (lang !== "es" || text == null) return null;
  const core = text.trim();
  if (!core || !Object.prototype.hasOwnProperty.call(PHRASES, core)) return null;
  return text.replace(core, PHRASES[core]);
}

// Apply the global phrase table to text nodes and to placeholder/title/aria-label
// attributes. Original English is stashed so switching back restores it exactly.
function applyPhrases(root, lang) {
  const el = root || document;
  const isEs = lang === "es";

  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      if (!node.nodeValue || !node.nodeValue.trim()) return NodeFilter.FILTER_REJECT;
      const parent = node.parentElement;
      if (!parent) return NodeFilter.FILTER_REJECT;
      const tag = parent.tagName;
      if (tag === "SCRIPT" || tag === "STYLE" || tag === "NOSCRIPT") return NodeFilter.FILTER_REJECT;
      if (parent.hasAttribute("data-i18n")) return NodeFilter.FILTER_REJECT;
      return NodeFilter.FILTER_ACCEPT;
    },
  });

  let node;
  while ((node = walker.nextNode())) {
    if (isEs) {
      if (node.__i18nEs != null) continue;
      const translated = phraseFor(node.nodeValue, "es");
      if (translated != null) {
        node.__i18nEn = node.nodeValue;
        node.__i18nEs = translated;
        node.nodeValue = translated;
      }
    } else if (node.__i18nEn != null && node.nodeValue === node.__i18nEs) {
      node.nodeValue = node.__i18nEn;
      node.__i18nEn = null;
      node.__i18nEs = null;
    }
  }

  el.querySelectorAll("[placeholder], [title], [aria-label]").forEach((elem) => {
    ["placeholder", "title", "aria-label"].forEach((attr) => {
      if (!elem.hasAttribute(attr)) return;
      const key = "data-i18n-" + (attr === "aria-label" ? "aria" : attr);
      if (elem.hasAttribute(key)) return;
      const store = "__i18nEn_" + attr;
      if (isEs) {
        if (elem[store] != null) return;
        const value = elem.getAttribute(attr);
        const translated = phraseFor(value, "es");
        if (translated != null) {
          elem[store] = value;
          elem.setAttribute(attr, translated);
        }
      } else if (elem[store] != null) {
        elem.setAttribute(attr, elem[store]);
        elem[store] = null;
      }
    });
  });
}

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
