import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState, TextField, TextareaField } from "../../components/ui";
import { api } from "../../api";
import type { Warehouse, CreateWarehouseInput, UpdateWarehouseInput } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

export function WarehouseForm({ tabId, entryId, initialTitle }: Props) {
  const { markUnsaved, replaceDraft } = useWorkbench();
  const isEdit = entryId !== undefined && entryId !== null && entryId !== "";

  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  const [isActive, setIsActive] = useState(true);

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isEdit && entryId) {
      setSaving(true);
      api.getWarehouse(entryId as number)
        .then((wh) => {
          setCode(wh.code);
          setName(wh.name);
          setAddress(wh.address || "");
          setCity(wh.city || "");
          setIsActive(wh.is_active);
        })
        .catch((err) => setError(err instanceof Error ? err.message : "Failed to load warehouse"))
        .finally(() => setSaving(false));
    } else {
      replaceDraft(tabId, "New Warehouse", "DRAFT");
    }
  }, [isEdit, entryId, tabId, replaceDraft]);

  useEffect(() => {
    if (!isEdit) {
      markUnsaved(tabId, code !== "" || name !== "" || address !== "" || city !== "");
    }
  }, [code, name, address, city, isEdit, tabId, markUnsaved]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSaving(true);

    try {
      if (isEdit && entryId) {
        await api.updateWarehouse(entryId as number, {
          code,
          name,
          address,
          city,
          is_active: isActive,
        });
      } else {
        await api.createWarehouse({
          code,
          name,
          address,
          city,
          is_active: isActive,
        });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save warehouse.");
    } finally {
      setSaving(false);
    }
  };

  if (saving && isEdit) {
    return <LoadingState label="Loading warehouse..." />;
  }

  return (
    <div className="entrytab entrytab--accurate">
      <form
        className="entrytab__main"
        onSubmit={handleSubmit}
        aria-label={isEdit ? "Edit warehouse form" : "Create new warehouse"}
      >
        <fieldset>
          <legend className="visually-hidden">Warehouse Details</legend>

          <div className="grid-col-2">
            <TextField
              label="Code *"
              id="warehouse-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              required
              placeholder="e.g., WH-001"
            />
            <TextField
              label="Name *"
              id="warehouse-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="e.g., Main Warehouse"
            />
          </div>

          <TextareaField
            label="Address"
            id="warehouse-address"
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            placeholder="Street address..."
            rows={3}
          />

          <TextField
            label="City"
            id="warehouse-city"
            value={city}
            onChange={(e) => setCity(e.target.value)}
            placeholder="City"
          />

          <div className="form-actions">
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={isActive}
                onChange={(e) => setIsActive(e.target.checked)}
              />
              <span>Active</span>
            </label>
          </div>
        </fieldset>

        <aside className="action-rail">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
            {saving ? "Saving..." : isEdit ? "Save Changes" : "Save"}
          </button>
        </aside>
        <FormError message={error} />
      </form>
    </div>
  );
}
