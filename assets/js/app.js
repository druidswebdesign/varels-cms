// Frontend glue for HTMX + Alpine.js + ApexCharts.
//
// HTMX swaps DOM fragments server-side; Alpine v3 initialises the swapped tree
// (its MutationObserver would also do this, but we init synchronously inside
// htmx:afterSwap so new Alpine components are live before the next paint).
// Failed HTMX requests are surfaced as toasts in the #flash region.

// Re-initialise Alpine on swapped content and let chart partials re-render.
document.addEventListener("htmx:afterSwap", (event) => {
  const target = event.detail && event.detail.target;
  if (window.Alpine && target && !target._x_dataStack) {
    window.Alpine.initTree(target);
  }
  window.dispatchEvent(new CustomEvent("app:swap", { detail: { target } }));
});

// Toast a failed HTMX request (e.g. 4xx/5xx that did not swap).
document.addEventListener("htmx:responseError", (event) => {
  const status = event.detail && event.detail.xhr ? event.detail.xhr.status : "?";
  showFlash(`Request failed (${status})`, "error");
});

// Toast network / connection failures.
document.addEventListener("htmx:sendError", () => {
  showFlash("Network error — check your connection.", "error");
});

// showFlash prepends a dismissible-by-timeout toast into the #flash slot.
function showFlash(message, tone) {
  const host = document.getElementById("flash");
  if (!host) return;
  const el = document.createElement("div");
  el.setAttribute("role", "alert");
  el.className =
    tone === "error"
      ? "mb-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800"
      : "mb-2 rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-800";
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
