"""Inventory every className referenced in TSX + every class defined in CSS.
Outputs the coverage gap: classes used in TSX but not defined in CSS.
"""
import re
from collections import Counter
from pathlib import Path

root = Path("src")

used = Counter()
for p in root.rglob("*.tsx"):
    text = p.read_text()
    # static className="..."
    for m in re.finditer(r'className="([^"]+)"', text):
        for c in m.group(1).split():
            used[c] += 1
    # template literals with static prefix: className={`foo ${...}`}
    for m in re.finditer(r"className=\{`([^`$]*)", text):
        for c in m.group(1).split():
            if c and "$" not in c:
                used[c] += 1
    # ternaries with static strings
    for m in re.finditer(r'className=\{[^}]*?"([\w -]+)"[^}]*?\}', text):
        for c in m.group(1).split():
            used[c] += 1

defined = set()
for p in root.rglob("*.css"):
    text = p.read_text()
    for m in re.finditer(r"\.([\w-]+)", text):
        defined.add(m.group(1))

out = []
out.append(f"=== {len(used)} distinct classes used in TSX ===")
gap = sorted(c for c in used if c not in defined)
out.append(f"=== used but NOT defined in CSS: {len(gap)} ===")
out += gap
out.append("")
out.append("=== all used classes with counts ===")
for k, v in sorted(used.items(), key=lambda x: -x[1]):
    marker = "" if k in defined else "  <-- MISSING"
    out.append(f"{v:4d}  {k}{marker}")

Path("/tmp/class-inventory.txt").write_text("\n".join(out))
print(f"used: {len(used)}, missing: {len(gap)}; report at /tmp/class-inventory.txt")
