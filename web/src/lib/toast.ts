/**
 * Minimal toast notifications.
 *
 * The app has no global notification system; reports need one for export
 * failures. This renders into a fixed live region (#toast-region) appended to
 * <body> on first use, auto-dismisses after a few seconds, and stays free of
 * React state so it can be called from any async handler.
 */

export type ToastKind = "success" | "error";

const TOAST_TTL_MS = 4200;
const TOAST_EXIT_MS = 200;

export function showToast(message: string, kind: ToastKind = "success"): void {
  if (typeof document === "undefined") return;

  let region = document.getElementById("toast-region");
  if (!region) {
    region = document.createElement("div");
    region.id = "toast-region";
    region.className = "toast-region";
    region.setAttribute("aria-live", "polite");
    document.body.appendChild(region);
  }

  const toast = document.createElement("div");
  toast.className = `toast toast--${kind}`;
  toast.setAttribute("role", kind === "error" ? "alert" : "status");
  toast.textContent = message;
  region.appendChild(toast);

  window.setTimeout(() => {
    toast.classList.add("is-leaving");
    window.setTimeout(() => toast.remove(), TOAST_EXIT_MS);
  }, TOAST_TTL_MS);
}
