"""Survey edge cases for the button codemod — linear scan, no backtracking regex."""
from pathlib import Path
from collections import Counter
import re

root = Path("src")
out = []

# Iterate over every opening <button ...> tag via a simple split-based scan.
def iter_tags(text, tag):
    # yields (start_index, full_tag_string)
    for m in re.finditer(r"<" + tag + r"[\s>]", text):
        start = m.start()
        end = text.find(">", start)
        if end == -1:
            continue
        yield start, text[start:end + 1]

btn_tags = []
other_class_btn = Counter()
link_btn = []
submit_sites = []
dynamic_class = []

for p in root.rglob("*.tsx"):
    text = p.read_text()
    for start, tag in iter_tags(text, "button"):
        if re.search(r'className="btn', tag):
            btn_tags.append((p, start, tag))
        else:
            cm = re.search(r'className="([^"]*)"', tag)
            if cm and "btn" in cm.group(1):
                other_class_btn[cm.group(1)] += 1
    for start, tag in iter_tags(text, "a"):
        if re.search(r'className="[^"]*btn', tag):
            link_btn.append((p, text[:start].count("\n") + 1, tag[:160]))
    # submit buttons with btn class
    for p2 in [p]:
        for start, tag in iter_tags(text, "button"):
            if 'type="submit"' in tag and re.search(r'className="btn', tag):
                submit_sites.append(str(p2))
    # dynamic className referencing btn
    for i, line in enumerate(text.splitlines(), 1):
        if "className=" in line and "btn" in line:
            stripped = line.strip()
            if 'className="btn' in stripped:
                continue
            if "`btn" in line or '"btn ' in line or "'btn" in line or "btn--" in line:
                dynamic_class.append(f"{p}:{i}: {stripped[:140]}")

out.append(f"=== .btn button tags found: {len(btn_tags)} ===")
attrs = Counter()
for p, start, tag in btn_tags:
    for a in re.findall(r'\s([a-zA-Z-]+)(?:=|\s|/|$)', tag):
        attrs[a] += 1
out.append("--- attribute frequency on .btn tags ---")
for k, v in sorted(attrs.items(), key=lambda x: -x[1]):
    out.append(f"{v:4d}  {k}")

out.append("\n=== submit .btn sites: " + str(len(submit_sites)) + " ===")
out += submit_sites[:20]

out.append("\n=== <a ...btn class sites: " + str(len(link_btn)) + " ===")
out += [f"{p}:{ln}: {t[:120]}" for p, ln, t in link_btn[:20]]

out.append("\n=== other button classNames containing 'btn': ===")
for k, v in other_class_btn.most_common(20):
    out.append(f"{v:4d}  {k}")

out.append("\n=== dynamic className lines mentioning btn: " + str(len(dynamic_class)) + " ===")
out += dynamic_class[:40]

Path("/tmp/btn-edge.txt").write_text("\n".join(out))
print("OK")
