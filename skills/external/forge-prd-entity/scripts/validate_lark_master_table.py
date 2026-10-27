#!/usr/bin/env python3
"""Check native PRD tables against the selected layout, not a fixed template.

This checks structure only, not product semantics, actual rendering, permissions,
image freshness, or preservation of concurrent edits. The historical filename
also supports architecture-grouped tables.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import xml.etree.ElementTree as ET
from pathlib import Path
from collections.abc import Sequence


EXPLANATION_LABELS = (
    "功能与效果", "输入要求", "可选配置", "页面目标", "核心组件",
    "交互逻辑", "状态与异常", "验收", "功能说明",
)
VISUAL_LABELS = {"原型图", "页面截图", "界面示例", "prototype", "screenshot"}
PLACEHOLDER_RE = re.compile(r"\bTODO\b|待补充|\[\s*\.\.\.\s*\]", re.IGNORECASE)


def _parse(text: str) -> ET.Element:
    text = re.sub(r"^\s*<\?xml[^>]*>\s*", "", text)
    try:
        return ET.fromstring(text)
    except ET.ParseError:
        return ET.fromstring(f"<root>{text}</root>")


def _text(node: ET.Element) -> str:
    return " ".join(part.strip() for part in node.itertext() if part.strip()).strip()


def _heading_title(value: str) -> str:
    return re.sub(r"^\d+(?:\.\d+)*[.)、]?\s+", "", value).strip()


def _rows(table: ET.Element) -> list[ET.Element]:
    return table.findall("./tbody/tr") or table.findall("./tr")


def _table_headers(table: ET.Element) -> list[str]:
    row = table.find("./thead/tr")
    if row is None:
        rows = _rows(table)
        row = rows[0] if rows else None
    if row is None:
        return []
    return [_text(cell) for cell in row if cell.tag in {"th", "td"}]


def _is_design_table(table: ET.Element) -> bool:
    headers = _table_headers(table)
    return (
        len(headers) == 2
        and any(word in headers[0] for word in ("模块", "功能", "页面"))
        and any(word in headers[1] for word in ("原型图", "页面截图"))
    )


def _design_tables(root: ET.Element) -> list[tuple[ET.Element, str]]:
    found: list[tuple[ET.Element, str]] = []
    last_heading = ""

    def visit(node: ET.Element) -> None:
        nonlocal last_heading
        if node.tag in {"h1", "h2", "h3"}:
            last_heading = _heading_title(_text(node))
            return
        if node.tag == "table":
            if _is_design_table(node):
                found.append((node, last_heading))
            # Cell headings and nested tables must not become branch headings.
            return
        for child in node:
            visit(child)

    visit(root)
    return found


def _data_rows(table: ET.Element) -> list[ET.Element]:
    rows = _rows(table)
    if table.find("./thead/tr") is None and rows:
        return rows[1:]
    return rows


def _explanation_value(value: str) -> str:
    value = value.strip()
    if value.lower() in VISUAL_LABELS:
        return ""
    for label in EXPLANATION_LABELS:
        if value.startswith(label):
            value = value[len(label):].lstrip(" :：—-")
            break
    return value


def _substantive(value: str) -> bool:
    if not value or PLACEHOLDER_RE.search(value):
        return False
    without_labels = re.sub("|".join(map(re.escape, EXPLANATION_LABELS)), "", value)
    if not re.search(r"[\w\u4e00-\u9fff]", without_labels):
        return False
    return len(re.findall(r"[\u4e00-\u9fff]", without_labels)) >= 4 or len(re.findall(r"\b[A-Za-z][\w-]*\b", without_labels)) >= 3


def validate_xml(
    text: str,
    expected_modules: int = 0,
    *,
    expected_tables: int | None = None,
    expected_sections: Sequence[str] = (),
    required_labels: Sequence[str] = (),
    require_visuals: bool = False,
    visual_modules: Sequence[str] = (),
    hierarchical_labels: bool = False,
) -> dict:
    if expected_modules < 0 or (expected_tables is not None and expected_tables < 1):
        raise ValueError("table count must be positive; module count cannot be negative")
    candidates = _design_tables(_parse(text))
    errors: list[str] = []
    if not candidates:
        errors.append("no native two-column product-design table found")
    if expected_tables is not None and len(candidates) != expected_tables:
        errors.append(f"expected {expected_tables} design tables, found {len(candidates)}")
    section_titles = [title for _, title in candidates]
    if expected_sections and section_titles != list(expected_sections):
        errors.append(f"expected section order {list(expected_sections)}, found {section_titles}")

    modules: list[str] = []
    identities: list[tuple[str, str, str]] = []
    row_results: list[dict] = []
    matched_visuals: set[str] = set()
    for table_number, (table, section) in enumerate(candidates, start=1):
        rows = _data_rows(table)
        if not rows:
            errors.append(f"table {table_number} ({section}): no feature rows")
        for row_number, row in enumerate(rows, start=1):
            cells = [cell for cell in row if cell.tag in {"td", "th"}]
            prefix = f"table {table_number} row {row_number}"
            if len(cells) != 2:
                errors.append(f"{prefix}: expected 2 cells, found {len(cells)}")
                continue
            left, right = cells
            title = left.find("./h4")
            hierarchy_node = left.find("./h3")
            module = _text(title) if title is not None else _text(left)
            hierarchy = _text(hierarchy_node) if hierarchy_node is not None else ""
            modules.append(module)
            identities.append((section, hierarchy, module))
            row_errors: list[str] = []
            if not module:
                row_errors.append("missing feature name")
            if left.find(".//img") is not None:
                row_errors.append("image found in left cell")
            if hierarchical_labels:
                visible = [child for child in left if _text(child)]
                if not hierarchy or not visible or visible[0].tag != "h3":
                    row_errors.append("functional hierarchy must be the first visible H3 in left cell")
                if title is None or not module:
                    row_errors.append("feature name must be an H4 in left cell")

            selectors = {selector for selector in visual_modules if selector in {module, _text(left)}}
            matched_visuals.update(selectors)
            images = list(right.iter("img"))
            if (require_visuals or selectors) and not images:
                row_errors.append("required UI image missing from right cell")
            for img in images:
                src = img.get("src", "").strip()
                if not src or src.startswith("[") or PLACEHOLDER_RE.search(src):
                    row_errors.append("image has no resolved source/token")

            image_positions: list[int] = []
            explanation_positions: list[int] = []
            explanation_values: list[str] = []
            label_positions: dict[str, int] = {}
            for index, child in enumerate(right):
                if child.tag == "img" or child.find(".//img") is not None:
                    image_positions.append(index)
                value = _text(child)
                for label in EXPLANATION_LABELS:
                    if value.startswith(label):
                        label_positions.setdefault(label, index)
                # Ignore headings/captions; require actual explanatory content.
                if child.tag not in {"img", "h1", "h2", "h3", "h4", "h5", "h6"}:
                    body = _explanation_value(value)
                    if _substantive(body):
                        explanation_positions.append(index)
                        explanation_values.append(body)
            if not explanation_values:
                row_errors.append("missing substantive explanation (labels alone do not count)")
            for label in required_labels:
                if label not in _text(right):
                    row_errors.append(f"missing agreed field {label}")
            prose_positions = explanation_positions + list(label_positions.values())
            if image_positions and prose_positions and max(image_positions) >= min(prose_positions):
                row_errors.append("embedded images must precede their explanation in right cell")
            if PLACEHOLDER_RE.search(_text(right)):
                row_errors.append("unresolved drafting placeholder")
            errors.extend(f"{prefix} ({module or 'unnamed'}): {item}" for item in row_errors)
            row_results.append({
                "table": table_number, "section": section, "row": row_number,
                "hierarchy": hierarchy, "module": module, "right_images": len(images),
                "present_labels": list(label_positions), "errors": row_errors,
            })

    if expected_modules and len(row_results) != expected_modules:
        errors.append(f"expected {expected_modules} feature rows, found {len(row_results)}")
    if len(identities) != len(set(identities)):
        errors.append("duplicate feature rows within the same section/hierarchy")
    for selector in set(visual_modules) - matched_visuals:
        errors.append(f"required visual module not found: {selector}")
    return {
        "ok": not errors, "table_count": len(candidates),
        "master_table_count": len(candidates),  # Compatibility with older callers.
        "sections": section_titles, "row_count": len(row_results),
        "modules": modules, "rows": row_results, "errors": errors,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("xml", help="DocxXML file path, or - for stdin")
    parser.add_argument("--expected-modules", type=int, default=0)
    parser.add_argument("--expected-tables", type=int, help="Agreed table count; no fixed default")
    parser.add_argument("--expected-section", action="append", default=[], help="Branch headings, in order")
    parser.add_argument("--required-label", action="append", default=[], help="Only fields agreed as mandatory")
    parser.add_argument("--require-visuals", action="store_true", help="Every row needs an image (explicit contract only)")
    parser.add_argument("--visual-module", action="append", default=[], help="A UI row title requiring an image")
    parser.add_argument("--hierarchical-labels", action="store_true", help="Check left-cell H3 hierarchy and H4 feature name")
    args = parser.parse_args()
    try:
        text = sys.stdin.read() if args.xml == "-" else Path(args.xml).read_text(encoding="utf-8")
        result = validate_xml(
            text, args.expected_modules, expected_tables=args.expected_tables,
            expected_sections=args.expected_section, required_labels=args.required_label,
            require_visuals=args.require_visuals, visual_modules=args.visual_module,
            hierarchical_labels=args.hierarchical_labels,
        )
    except (OSError, UnicodeDecodeError, ET.ParseError, ValueError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
