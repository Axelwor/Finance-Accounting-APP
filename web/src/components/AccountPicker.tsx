import { useMemo } from "react";
import { StaticCombobox } from "./Combobox";
import type { AccountItem } from "../types";

/**
 * AccountPicker — a searchable account combobox with optional type filtering.
 *
 * Thin wrapper over StaticCombobox that:
 *  - renders options as "code - name" (code is the fastest lookup key for
 *    bookkeepers),
 *  - optionally filters to a set of backend account_types (e.g. CASH/BANK
 *    for the cash side of a cash entry),
 *  - excludes a set of account ids (e.g. the From account of a transfer).
 *
 * Usage:
 *   <AccountPicker
 *     accounts={accounts}
 *     value={cashAccount}
 *     onChange={setCashAccount}
 *     allowedTypes={["CASH", "BANK"]}
 *     placeholder="Pilih kas / bank…"
 *   />
 */

export interface AccountPickerProps {
  accounts: AccountItem[];
  value: string | null;
  onChange: (value: string | null) => void;
  /** Only allow these backend account_types. Empty = all types. */
  allowedTypes?: string[];
  /** Exclude these account ids (e.g. the other side of a transfer). */
  excludeIds?: string[];
  placeholder?: string;
  disabled?: boolean;
  id?: string;
  "aria-label"?: string;
}

interface AccountOption {
  value: string;
  label: string;
  code?: string;
}

export function AccountPicker({
  accounts,
  value,
  onChange,
  allowedTypes,
  excludeIds,
  placeholder = "Ketik kode / nama akun…",
  disabled,
  id,
  "aria-label": ariaLabel,
}: AccountPickerProps) {
  const options = useMemo<AccountOption[]>(() => {
    const exclude = new Set(excludeIds ?? []);
    const types = allowedTypes && allowedTypes.length > 0 ? new Set(allowedTypes) : null;
    return accounts
      .filter((a) => !exclude.has(String(a.id)))
      .filter((a) => !types || (a.account_type ? types.has(a.account_type) : true))
      .map((a) => ({ value: String(a.id), label: a.name, code: a.code }));
  }, [accounts, allowedTypes, excludeIds]);

  return (
    <StaticCombobox
      options={options}
      value={value}
      onChange={(v) => onChange(v ?? "")}
      placeholder={placeholder}
      disabled={disabled}
      className="account-picker"
      id={id}
      aria-label={ariaLabel}
    />
  );
}

/** True when the account is a cash/bank account (by type when present). */
export function isCashBankAccount(account: AccountItem | undefined): boolean {
  if (!account) return false;
  return account.account_type === "CASH" || account.account_type === "BANK";
}
