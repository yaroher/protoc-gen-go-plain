#!/usr/bin/env python3
import json
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: extract_ir_type.py <type-name-substring> [ir_json_path]", file=sys.stderr)
        return 2
    needle = sys.argv[1]
    ir_path = Path(sys.argv[2]) if len(sys.argv) > 2 else Path("bin/json/ir.json")
    if not ir_path.exists():
        print(f"ir json not found: {ir_path}", file=sys.stderr)
        return 2
    with ir_path.open("r", encoding="utf-8") as f:
        data = json.load(f)

    matches = []
    for entry in data:
        file_name = entry.get("fileName")
        messages = entry.get("messages", {})
        crf_map = entry.get("crf", {}) or {}
        for name, msg in messages.items():
            if needle in name:
                matches.append((file_name, name, msg, crf_map.get(name)))

    if not matches:
        print(f"no matches for '{needle}' in {ir_path}")
        return 1

    # Prefer exact match if present
    exact = [m for m in matches if m[1] == needle]
    chosen = exact[0] if exact else matches[0]

    file_name, name, msg, crf_meta = chosen
    out = {
        "file": file_name,
        "name": name,
        "type": msg,
        "match_count": len(matches),
    }
    if crf_meta is not None:
        out["crf"] = crf_meta
    json.dump(out, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    if len(matches) > 1:
        sys.stderr.write(f"note: {len(matches)} matches; showing {name} (from {file_name})\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
