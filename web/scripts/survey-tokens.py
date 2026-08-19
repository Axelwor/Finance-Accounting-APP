"""Survey all CSS var() tokens in the styles directory."""
import re
from collections import Counter
from pathlib import Path

root = Path("src")
tokens = Counter()
files = Counter()

for p in root.rglob("*.css"):
    text = p.read_text()
    found = set(re.findall(r"var\((--[\w-]+)", text))
    for t in found:
        tokens[t] += 1
        files[t] += 1

out = []
out.append("=== distinct tokens in CSS (name, distinct files) ===")
for k, v in sorted(tokens.items()):
    out.append(f"{v:4d}  {k}")

# also inline styles in tsx using var(--...)
tsx_tokens = Counter()
for p in root.rglob("*.tsx"):
    text = p.read_text()
    for t in set(re.findall(r"var\((--[\w-]+)", text)):
        tsx_tokens[t] += 1
out.append("\n=== tokens in TSX inline styles ===")
for k, v in sorted(tsx_tokens.items()):
    out.append(f"{v:4d}  {k}")

Path("/tmp/token-survey.txt").write_text("\n".join(out))
print("written /tmp/token-survey.txt", len(tokens), "css tokens,", len(tsx_tokens), "tsx tokens")
