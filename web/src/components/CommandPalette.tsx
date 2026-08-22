import { useEffect, useMemo, useRef, useState } from "react";
import { useWorkbench } from "../workbench/state";
import { MODULES } from "../workbench/modules";
import { Icon } from "./m3/Icon";
import type { EntrySubKind, ListSubKind } from "../workbench/types";

interface CommandItem {
  id: string;
  category: "Navigasi Modul" | "Buat Dokumen Cepat";
  label: string;
  subLabel?: string;
  icon?: string;
  action: () => void;
}

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export function CommandPalette({ isOpen, onClose }: Props) {
  const workbench = useWorkbench();
  const [search, setSearch] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  // Build command catalogue
  const commands: CommandItem[] = useMemo(() => {
    const list: CommandItem[] = [
      {
        id: "quick-money-in",
        category: "Buat Dokumen Cepat",
        label: "+ Kas Masuk (Other Receipt)",
        subLabel: "Kas & Bank",
        icon: "arrow_down_left",
        action: () => workbench.openEntryDraft("money-in"),
      },
      {
        id: "quick-money-out",
        category: "Buat Dokumen Cepat",
        label: "- Kas Keluar (Other Payment)",
        subLabel: "Kas & Bank",
        icon: "arrow_up_right",
        action: () => workbench.openEntryDraft("money-out"),
      },
      {
        id: "quick-transfer",
        category: "Buat Dokumen Cepat",
        label: "Transfer Bank",
        subLabel: "Kas & Bank",
        icon: "refresh",
        action: () => workbench.openEntryDraft("cash-transfer"),
      },
      {
        id: "quick-sales-invoice",
        category: "Buat Dokumen Cepat",
        label: "+ Faktur Penjualan (Sales Invoice)",
        subLabel: "Penjualan",
        icon: "receipt",
        action: () => workbench.openEntryDraft("sales-invoice"),
      },
      {
        id: "quick-journal",
        category: "Buat Dokumen Cepat",
        label: "+ Jurnal Umum Manual",
        subLabel: "Buku Besar",
        icon: "book_open",
        action: () => workbench.openEntryDraft("journal-entry"),
      },
    ];

    // Modules subitems
    MODULES.forEach((mod) => {
      mod.items.forEach((item) => {
        list.push({
          id: `mod-${mod.id}-${item.id}`,
          category: "Navigasi Modul",
          label: `Buka ${item.label}`,
          subLabel: `${mod.label} › ${item.hint || ""}`,
          icon: mod.icon,
          action: () => {
            if (item.openList) {
              workbench.openList(item.openList as ListSubKind);
            } else if (item.openEntry) {
              workbench.openEntryDraft(item.openEntry as EntrySubKind);
            }
          },
        });
      });
    });

    return list;
  }, [workbench]);

  // Filter commands
  const filteredCommands = useMemo(() => {
    if (!search.trim()) return commands;
    const q = search.toLowerCase();
    return commands.filter(
      (c) =>
        c.label.toLowerCase().includes(q) ||
        (c.subLabel && c.subLabel.toLowerCase().includes(q)) ||
        c.category.toLowerCase().includes(q)
    );
  }, [commands, search]);

  // Focus on open
  useEffect(() => {
    if (isOpen) {
      setSearch("");
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [isOpen]);

  // Reset index when search changes
  useEffect(() => {
    setSelectedIndex(0);
  }, [search]);

  // Scroll active item into view
  useEffect(() => {
    if (listRef.current) {
      const activeEl = listRef.current.querySelector<HTMLElement>(".command-palette-item.is-selected");
      activeEl?.scrollIntoView({ block: "nearest" });
    }
  }, [selectedIndex]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((prev) => (prev + 1) % Math.max(1, filteredCommands.length));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((prev) =>
        prev === 0 ? Math.max(0, filteredCommands.length - 1) : prev - 1
      );
    } else if (e.key === "Enter") {
      e.preventDefault();
      const target = filteredCommands[selectedIndex];
      if (target) {
        target.action();
        onClose();
      }
    }
  };

  if (!isOpen) return null;

  return (
    <div
      className="command-palette-backdrop"
      style={{
        position: "fixed",
        inset: 0,
        backgroundColor: "rgba(15, 23, 42, 0.45)",
        backdropFilter: "blur(4px)",
        zIndex: 200,
        display: "flex",
        justifyContent: "center",
        alignItems: "flex-start",
        paddingTop: "15vh",
      }}
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label="Command Palette"
    >
      <div
        className="command-palette-card"
        style={{
          width: "100%",
          maxWidth: "560px",
          backgroundColor: "var(--bg-surface)",
          border: "1px solid var(--border-color)",
          borderRadius: "var(--radius-md)",
          boxShadow: "var(--shadow-xl)",
          overflow: "hidden",
          display: "flex",
          flexDirection: "column",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Search Input Bar */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "10px",
            padding: "12px 16px",
            borderBottom: "1px solid var(--border-color)",
            backgroundColor: "var(--bg-surface)",
          }}
        >
          <Icon name="search" size={20} className="text-muted" />
          <input
            ref={inputRef}
            type="text"
            className="command-palette-input"
            style={{
              flex: 1,
              border: "none",
              outline: "none",
              fontSize: "14px",
              color: "var(--text-primary)",
              backgroundColor: "transparent",
              fontFamily: "inherit",
            }}
            placeholder="Cari modul, transaksi, atau ketik perintah... (↑↓ untuk pilih, Enter untuk buka)"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <span
            style={{
              fontSize: "11px",
              fontFamily: "var(--font-mono)",
              color: "var(--text-muted)",
              padding: "2px 6px",
              backgroundColor: "var(--bg-surface-secondary)",
              borderRadius: "var(--radius-xs)",
              border: "1px solid var(--border-color)",
            }}
          >
            ESC
          </span>
        </div>

        {/* Command List */}
        <ul
          ref={listRef}
          role="listbox"
          style={{
            listStyle: "none",
            maxHeight: "360px",
            overflowY: "auto",
            padding: "6px",
            margin: 0,
          }}
        >
          {filteredCommands.length === 0 ? (
            <li
              style={{
                padding: "24px 16px",
                textAlign: "center",
                color: "var(--text-muted)",
                fontSize: "13px",
              }}
            >
              Tidak ditemukan perintah atau modul untuk &quot;{search}&quot;.
            </li>
          ) : (
            filteredCommands.map((cmd, idx) => {
              const isSelected = idx === selectedIndex;
              return (
                <li
                  key={cmd.id}
                  role="option"
                  aria-selected={isSelected}
                  className={`command-palette-item ${isSelected ? "is-selected" : ""}`}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    padding: "8px 12px",
                    borderRadius: "var(--radius-sm)",
                    backgroundColor: isSelected ? "var(--brand-primary-light)" : "transparent",
                    color: isSelected ? "var(--brand-primary)" : "var(--text-primary)",
                    cursor: "pointer",
                    transition: "background-color 0.1s ease",
                  }}
                  onClick={() => {
                    cmd.action();
                    onClose();
                  }}
                  onMouseEnter={() => setSelectedIndex(idx)}
                >
                  <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                    <span
                      style={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        color: isSelected ? "var(--brand-primary)" : "var(--text-muted)",
                      }}
                    >
                      <Icon name={cmd.icon || "arrow_forward"} size={16} />
                    </span>
                    <span style={{ fontSize: "13px", fontWeight: isSelected ? 600 : 500 }}>
                      {cmd.label}
                    </span>
                  </div>

                  {cmd.subLabel && (
                    <span
                      style={{
                        fontSize: "11px",
                        color: isSelected ? "var(--brand-primary)" : "var(--text-muted)",
                        fontFamily: "var(--font-mono)",
                      }}
                    >
                      {cmd.subLabel}
                    </span>
                  )}
                </li>
              );
            })
          )}
        </ul>

        {/* Footer info */}
        <div
          style={{
            padding: "8px 14px",
            backgroundColor: "var(--bg-surface-secondary)",
            borderTop: "1px solid var(--border-color)",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            fontSize: "11px",
            color: "var(--text-muted)",
          }}
        >
          <span>Pintasan Navigasi Cepat</span>
          <span>Tekan <strong>↵ Enter</strong> untuk eksekusi</span>
        </div>
      </div>
    </div>
  );
}
