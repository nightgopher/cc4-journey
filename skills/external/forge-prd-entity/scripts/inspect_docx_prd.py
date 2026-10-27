#!/usr/bin/env python3
"""Check the structure of two-column image-and-description tables in a .docx PRD.

Checks: design tables have two columns; images sit in the right cell before the
explanation; explanations are not empty; named UI rows have an image; optional
branch-heading order. It does not judge content, image clarity, or rendering.

Usage:
    python3 inspect_docx_prd.py prd.docx [--visual-row 导入任务列表 ...]
        [--expected-section 任务管理 --expected-section 结果管理] [--expected-tables 2]
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

try:
    from docx import Document
    from docx.oxml.ns import qn
except ImportError:  # pragma: no cover
    print("ERROR: python-docx is required (pip install python-docx)", file=sys.stderr)
    sys.exit(2)

LEFT_WORDS = ("功能", "模块", "页面")
RIGHT_WORDS = ("原型", "截图", "说明")


def el_text(el) -> str:
    return "".join(t.text or "" for t in el.iter(qn("w:t"))).strip()


def has_image(el) -> bool:
    return any(True for _ in el.iter(qn("w:drawing"))) or any(True for _ in el.iter(qn("w:pict")))


def substantive(text: str) -> bool:
    return len(re.findall(r"[一-鿿]", text)) >= 4 or len(re.findall(r"[A-Za-z]+", text)) >= 3


def cells_of(tr):
    return tr.findall(qn("w:tc"))


def inspect(path: Path, visual_rows: list[str], expected_sections: list[str], expected_tables: int | None) -> dict:
    doc = Document(str(path))
    body = doc.element.body
    errors: list[str] = []
    tables = []
    last_heading = ""
    style_names = {s.style_id: s.name for s in doc.styles}
    for child in body:
        if child.tag == qn("w:p"):
            ppr = child.find(qn("w:pPr"))
            sid = ppr.find(qn("w:pStyle")).get(qn("w:val")) if ppr is not None and ppr.find(qn("w:pStyle")) is not None else ""
            name = style_names.get(sid, sid)
            if re.match(r"(Heading|标题)\s*\d", name or "") and el_text(child):
                last_heading = re.sub(r"^\d+(?:\.\d+)*[.、]?\s*", "", el_text(child))
            continue
        if child.tag != qn("w:tbl"):
            continue
        trs = child.findall(qn("w:tr"))
        if not trs:
            continue
        header = [el_text(tc) for tc in cells_of(trs[0])]
        if len(header) != 2 or not any(w in header[0] for w in LEFT_WORDS) or not any(w in header[1] for w in RIGHT_WORDS):
            continue
        rows = []
        seen = set()
        for r_num, tr in enumerate(trs[1:], start=1):
            tcs = cells_of(tr)
            prefix = f"table {len(tables) + 1} row {r_num}"
            if len(tcs) != 2:
                errors.append(f"{prefix}: expected 2 cells, found {len(tcs)}")
                continue
            left, right = tcs
            left_lines = [el_text(p) for p in left.findall(qn("w:p")) if el_text(p)]
            name = left_lines[-1] if left_lines else ""
            hierarchy = left_lines[0] if len(left_lines) > 1 else ""
            row_err = []
            if not name:
                row_err.append("missing feature name in left cell")
            if has_image(left):
                row_err.append("image found in left cell")
            first_img = first_text = None
            explanation = ""
            img_count = 0
            for idx, block in enumerate(c for c in right if c.tag in (qn("w:p"), qn("w:tbl"))):
                if has_image(block):
                    img_count += sum(1 for _ in block.iter(qn("w:drawing")))
                    if first_img is None:
                        first_img = idx
                text = el_text(block)
                if text and not text.startswith("[缺少图片"):
                    explanation += text
                    if first_text is None:
                        first_text = idx
            if img_count and first_text is not None and first_text < first_img:
                row_err.append("text appears before the image in right cell")
            if not substantive(explanation):
                row_err.append("missing substantive explanation")
            if "[缺少图片" in el_text(right):
                row_err.append("missing image file placeholder")
            if name in visual_rows and not img_count:
                row_err.append("required UI image missing")
            if (hierarchy, name) in seen:
                row_err.append("duplicate feature row")
            seen.add((hierarchy, name))
            errors.extend(f"{prefix} ({name or 'unnamed'}): {e}" for e in row_err)
            rows.append({"hierarchy": hierarchy, "name": name, "images": img_count,
                         "explanation_chars": len(explanation), "errors": row_err})
        tables.append({"section": last_heading, "rows": rows})
    if not tables:
        errors.append("no two-column design table found")
    found_names = {r["name"] for t in tables for r in t["rows"]}
    for name in visual_rows:
        if name not in found_names:
            errors.append(f"required visual row not found: {name}")
    sections = [t["section"] for t in tables]
    if expected_sections and sections != expected_sections:
        errors.append(f"expected section order {expected_sections}, found {sections}")
    if expected_tables is not None and len(tables) != expected_tables:
        errors.append(f"expected {expected_tables} design tables, found {len(tables)}")
    return {"ok": not errors, "table_count": len(tables), "sections": sections,
            "row_count": sum(len(t["rows"]) for t in tables), "tables": tables, "errors": errors}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("docx", type=Path)
    parser.add_argument("--visual-row", action="append", default=[], help="Row name that must have an image")
    parser.add_argument("--expected-section", action="append", default=[], help="Branch heading before each table, in order")
    parser.add_argument("--expected-tables", type=int)
    args = parser.parse_args()
    try:
        result = inspect(args.docx, args.visual_row, args.expected_section, args.expected_tables)
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
