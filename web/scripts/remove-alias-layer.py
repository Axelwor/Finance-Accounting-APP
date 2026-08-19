"""Alias-layer removal: replace every var(--legacy-token) with the direct M3
token or literal, across CSS, TSX, and TS files. Handles var(--t, fallback)
forms by dropping the fallback (M3 tokens are always defined).

After running, delete the alias block from base.css and re-verify with
survey-tokens.py (anything not --md-* remaining needs review).
"""
import re
import sys
from pathlib import Path

WRITE = "--write" in sys.argv

# Legacy token -> replacement (--md-* token or CSS literal)
MAP = {
    # direct wave aliases
    "--canvas": "--md-sys-color-surface",
    "--paper": "--md-sys-color-surface-container-lowest",
    "--paper-elev": "--md-sys-color-surface-container-low",
    "--panel": "--md-sys-color-surface-container",
    "--panel-hover": "--md-sys-color-surface-container-high",
    "--panel-muted": "--md-sys-color-surface-container-highest",
    "--rule": "--md-sys-color-outline-variant",
    "--rule-soft": "--md-sys-color-surface-variant",
    "--rule-strong": "--md-sys-color-outline",
    "--ink-deep": "--md-sys-color-on-surface",
    "--ink": "--md-sys-color-on-surface-variant",
    "--ink-secondary": "--md-sys-color-on-surface-variant",
    "--ink-tertiary": "--md-sys-color-outline",
    "--ink-muted": "--md-sys-color-outline",
    "--ink-muted-bg": "--md-sys-color-surface-container",
    "--ink-faint": "--md-sys-color-outline-variant",
    "--accent-deep": "--md-sys-color-primary",
    "--accent": "--md-sys-color-primary",
    "--accent-hover": "--md-sys-color-primary",
    "--accent-soft": "--md-sys-color-primary-container",
    "--accent-rule": "--md-sys-color-secondary-container",
    "--positive": "--md-sys-color-success",
    "--positive-soft": "--md-sys-color-success-container",
    "--negative": "--md-sys-color-error",
    "--negative-soft": "--md-sys-color-error-container",
    "--warning": "--md-sys-color-warning",
    "--warning-soft": "--md-sys-color-warning-container",
    "--font-display": "--md-ref-typeface-brand",
    "--font-sans": "--md-ref-typeface-plain",
    "--font-mono": "--md-ref-typeface-plain",
    "--text-2xs": "0.6875rem",
    "--text-xs": "0.75rem",
    "--text-sm": "0.875rem",
    "--text-md": "0.9375rem",
    "--text-base": "1rem",
    "--text": "--md-sys-color-on-surface",
    "--text-secondary": "--md-sys-color-on-surface-variant",
    "--text-muted": "--md-sys-color-on-surface-variant",
    "--muted": "--md-sys-color-on-surface-variant",
    "--border": "--md-sys-color-outline-variant",
    "--line": "--md-sys-color-outline-variant",
    "--u-1": "--md-sys-spacing-1",
    "--u-2": "--md-sys-spacing-2",
    "--u-3": "--md-sys-spacing-3",
    "--u-4": "--md-sys-spacing-4",
    "--u-5": "--md-sys-spacing-5",
    "--u-6": "--md-sys-spacing-6",
    "--u-7": "--md-sys-spacing-7",
    "--u-8": "--md-sys-spacing-8",
    "--radius-xs": "2px",
    "--radius-sm": "--md-sys-shape-corner-extra-small",
    "--radius-md": "--md-sys-shape-corner-medium",
    "--radius-pill": "--md-sys-shape-corner-full",
    "--shadow-card": "--md-sys-elevation-1",
    "--shadow-pop": "--md-sys-elevation-3",
    # workbench-layer aliases (indirect)
    "--surface-canvas": "--md-sys-color-surface",
    "--surface-panel": "--md-sys-color-surface-container-lowest",
    "--surface-well": "--md-sys-color-surface-container",
    "--surface-elev": "--md-sys-color-surface-container-low",
    "--surface-hover": "--md-sys-color-surface-container-high",
    "--ink-primary": "--md-sys-color-on-surface",
    "--ink-dim": "--md-sys-color-outline-variant",
    "--rule-bright": "--md-sys-color-outline",
    "--pos": "--md-sys-color-success",
    "--pos-soft": "--md-sys-color-success-container",
    "--neg": "--md-sys-color-error",
    "--neg-soft": "--md-sys-color-error-container",
    "--acc": "--md-sys-color-primary",
    "--acc-soft": "--md-sys-color-primary-container",
    "--warn": "--md-sys-color-warning",
    "--warn-soft": "--md-sys-color-warning-container",
    "--space-1": "--md-sys-spacing-1",
    "--space-2": "--md-sys-spacing-2",
    "--space-3": "--md-sys-spacing-3",
    "--space-4": "--md-sys-spacing-4",
    "--space-5": "--md-sys-spacing-5",
    "--space-6": "--md-sys-spacing-6",
    "--space-7": "--md-sys-spacing-7",
    "--space-8": "--md-sys-spacing-8",
}

# longest first so --accent-deep beats --accent, --ink-deep beats --ink etc.
TOKENS = sorted(MAP.keys(), key=len, reverse=True)
PATTERNS = [(re.compile(r"var\(" + re.escape(t) + r"(?:\s*,\s*[^)]*)?\)"), MAP[t]) for t in TOKENS]

root = Path("src")
targets = sorted(list(root.rglob("*.css")) + list(root.rglob("*.tsx")) + list(root.rglob("*.ts")))
total_files = 0
total_repl = 0
changed = []

for p in targets:
    text = p.read_text()
    out = text
    count = 0
    for pat, repl in PATTERNS:
        def sub(m):
            return repl if repl.startswith("--") else repl
        out, n = pat.subn(lambda m: (("var(" + repl + ")") if repl.startswith("--") else repl), out)
        count += n
    if count:
        total_files += 1
        total_repl += count
        changed.append(f"{p}: {count}")
        if WRITE:
            p.write_text(out)

report = [f"files: {total_files}, replacements: {total_repl}, mode: {'WRITE' if WRITE else 'DRY RUN'}"]
report += changed
Path("/tmp/alias-removal-report.txt").write_text("\n".join(report))
print(f"done: {total_files} files, {total_repl} replacements ({'WRITE' if WRITE else 'DRY RUN'})")
