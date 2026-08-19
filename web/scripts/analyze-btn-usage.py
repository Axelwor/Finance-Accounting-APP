"""Analyze .btn usage patterns across screens for the M3 button codemod."""
import re
from collections import Counter
from pathlib import Path

root = Path("src")
patterns = Counter()
filecount = {}

for p in root.rglob("*.tsx"):
    text = p.read_text()
    found = re.findall(r'className="btn[^"]*"', text)
    if found:
        filecount[str(p)] = len(found)
        for m in found:
            patterns[m] += 1

out = []
out.append("=== class combos ===")
for k, v in patterns.most_common(50):
    out.append(f"{v:4d}  {k}")
out.append(f"total: {sum(patterns.values())}, files: {len(filecount)}")
out.append("")
out.append("=== also survey: <button with other classes used as buttons ===")
btn_tag = Counter()
for p in root.rglob("*.tsx"):
    text = p.read_text()
    for m in re.finditer(r'<button[^>]*className="([^"]*)"', text):
        btn_tag[m.group(1)] += 1
for k, v in btn_tag.most_common(30):
    out.append(f"{v:4d}  {k}")

Path("/tmp/btn-analysis.txt").write_text("\n".join(out))
print("written /tmp/btn-analysis.txt")
