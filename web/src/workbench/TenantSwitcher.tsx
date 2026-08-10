import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { useAppState } from "../state";
import type { Tenant } from "../types";

/**
 * Tenant (book) switcher shown in the top bar. Lists every book the user
 * belongs to, lets them switch between books, or open the form to create a
 * new book. Switching reloads the page so all cached data re-binds to the
 * newly-active tenant.
 */
export function TenantSwitcher() {
  const { business, setBusiness } = useAppState();
  const [open, setOpen] = useState(false);
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open && tenants.length === 0) {
      api.listTenants().then(setTenants);
    }
  }, [open, tenants.length]);

  // Close the dropdown when clicking outside.
  useEffect(() => {
    if (!open) return;
    const onMouseDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
        setCreating(false);
      }
    };
    document.addEventListener("mousedown", onMouseDown);
    return () => document.removeEventListener("mousedown", onMouseDown);
  }, [open]);

  const handleSwitch = async (tenant: Tenant) => {
    if (busy || tenant.id === business?.id) return;
    setBusy(true);
    try {
      const { business: switched } = await api.switchTenant(tenant);
      setBusiness(switched);
      // Full reload so every cached list/report re-binds to the new tenant.
      window.location.reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not switch business.");
      setBusy(false);
    }
  };

  const handleCreate = async () => {
    if (busy || !newName.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const { business: created } = await api.createTenant(newName);
      setBusiness(created);
      window.location.reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the business.");
      setBusy(false);
    }
  };

  return (
    <div className="tenant-switcher" ref={rootRef}>
      <button
        type="button"
        className="btn btn--ghost btn--sm tenant-switcher__toggle"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        title="Switch or add a business"
      >
        {business?.name || "Your business"} ▾
      </button>

      {open && (
        <div className="tenant-switcher__menu" role="menu">
          {tenants.map((tenant) => (
            <button
              key={tenant.id}
              type="button"
              role="menuitem"
              className={`tenant-switcher__item${tenant.id === business?.id ? " is-active" : ""}`}
              onClick={() => handleSwitch(tenant)}
              disabled={busy}
            >
              <span className="tenant-switcher__item-name">{tenant.name}</span>
              <span className="tenant-switcher__item-role">{tenant.role}</span>
            </button>
          ))}

          {tenants.length === 0 && !creating && (
            <p className="tenant-switcher__empty">No other businesses.</p>
          )}

          {!creating ? (
            <button
              type="button"
              className="tenant-switcher__add"
              onClick={() => setCreating(true)}
              disabled={busy}
            >
              + Add business
            </button>
          ) : (
            <div className="tenant-switcher__form">
              <input
                type="text"
                value={newName}
                placeholder="Business name"
                onChange={(e) => setNewName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleCreate();
                  if (e.key === "Escape") setCreating(false);
                }}
                autoFocus
                disabled={busy}
              />
              <div className="tenant-switcher__form-actions">
                <button
                  type="button"
                  className="btn btn--primary btn--sm"
                  onClick={handleCreate}
                  disabled={busy || !newName.trim()}
                >
                  {busy ? "Creating…" : "Create"}
                </button>
                <button
                  type="button"
                  className="btn btn--ghost btn--sm"
                  onClick={() => setCreating(false)}
                  disabled={busy}
                >
                  Cancel
                </button>
              </div>
            </div>
          )}

          {error && <p className="tenant-switcher__error">{error}</p>}
        </div>
      )}
    </div>
  );
}
