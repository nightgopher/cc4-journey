#!/usr/bin/env python3
"""Capture settled screenshots of prototype or product pages listed in a manifest.

Waits for load, network idle (best effort), fonts, image decoding, and a layout
that stays unchanged across consecutive checks before taking each screenshot.

Usage:
    python3 capture_screens.py manifest.json out_dir/
    python3 capture_screens.py --url https://example.com --name 首页 out_dir/

Manifest format:
    {
      "viewport": {"width": 1440, "height": 900},
      "device_scale_factor": 2,
      "screens": [
        {"name": "导入任务列表", "file": "01-导入任务列表.html"},
        {"name": "详情", "url": "https://...", "full_page": true,
         "selector": "#main", "wait_for": ".table", "viewport": {"width": 1600, "height": 1000}}
      ]
    }
Relative "file" paths resolve against the manifest's directory.
Requires Playwright with a Chromium browser. Pages behind a login should be
captured with an interactive browser tool instead.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import time
from pathlib import Path

LAYOUT_SIGNATURE_JS = """
() => {
  const doc = document.documentElement;
  const nodes = Array.from(document.querySelectorAll('body *')).slice(0, 400);
  let sig = [doc.scrollWidth, doc.scrollHeight, nodes.length].join(',');
  for (const el of nodes) {
    const r = el.getBoundingClientRect();
    sig += '|' + Math.round(r.x) + ',' + Math.round(r.y) + ',' + Math.round(r.width) + ',' + Math.round(r.height);
  }
  const text = (document.body ? document.body.innerText : '').length;
  return sig + '#' + text;
}
"""

READY_JS = """
async () => {
  if (document.fonts && document.fonts.ready) { await document.fonts.ready; }
  const imgs = Array.from(document.images);
  await Promise.all(imgs.map(img => img.complete ? (img.decode ? img.decode().catch(() => {}) : null)
    : new Promise(res => { img.addEventListener('load', res, {once: true}); img.addEventListener('error', res, {once: true}); })));
  return imgs.filter(img => !img.complete || img.naturalWidth === 0).length;
}
"""


def safe_name(value: str) -> str:
    value = re.sub(r"[\\/:*?\"<>|\s]+", "-", value.strip())
    return value.strip("-") or "screen"


def wait_until_settled(page, interval: float, needed: int, timeout: float) -> tuple[bool, int]:
    deadline = time.monotonic() + timeout
    last = None
    same = 0
    checks = 0
    while time.monotonic() < deadline:
        sig = page.evaluate(LAYOUT_SIGNATURE_JS)
        checks += 1
        if sig == last:
            same += 1
            if same >= needed:
                return True, checks
        else:
            same = 0
            last = sig
        time.sleep(interval)
    return False, checks


def load_manifest(args) -> tuple[dict, Path]:
    if args.url:
        return {"screens": [{"name": args.name or "screen", "url": args.url, "full_page": args.full_page}]}, Path.cwd()
    path = Path(args.manifest)
    return json.loads(path.read_text(encoding="utf-8")), path.parent


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("manifest", nargs="?", help="Manifest JSON path")
    parser.add_argument("out_dir", help="Output directory for PNG files and capture-report.json")
    parser.add_argument("--url", help="Capture a single URL instead of a manifest")
    parser.add_argument("--name", help="Screen name for --url")
    parser.add_argument("--full-page", action="store_true", help="Full-page capture for --url")
    parser.add_argument("--width", type=int, default=1440)
    parser.add_argument("--height", type=int, default=900)
    parser.add_argument("--scale", type=float, default=None, help="Device scale factor (default: manifest or 2)")
    parser.add_argument("--settle-timeout", type=float, default=15.0)
    parser.add_argument("--executable-path", default=os.environ.get("CHROMIUM_EXECUTABLE"))
    args = parser.parse_args()
    if not args.url and not args.manifest:
        parser.error("provide a manifest or --url")

    try:
        from playwright.sync_api import sync_playwright, TimeoutError as PWTimeout
    except ImportError:
        print("ERROR: playwright is not installed (pip install playwright)", file=sys.stderr)
        return 2

    manifest, base = load_manifest(args)
    default_vp = manifest.get("viewport") or {"width": args.width, "height": args.height}
    scale = args.scale or manifest.get("device_scale_factor") or 2
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    report = []

    with sync_playwright() as pw:
        launch = {"executable_path": args.executable_path} if args.executable_path else {}
        browser = pw.chromium.launch(**launch)
        for index, screen in enumerate(manifest.get("screens", []), start=1):
            name = screen.get("name") or f"screen-{index}"
            entry = {"name": name}
            if screen.get("file"):
                target = (base / screen["file"]).resolve()
                if not target.exists():
                    entry["error"] = f"file not found: {target}"
                    report.append(entry)
                    continue
                url = target.as_uri()
            else:
                url = screen.get("url", "")
            vp = screen.get("viewport") or default_vp
            context = browser.new_context(viewport={"width": vp["width"], "height": vp["height"]}, device_scale_factor=scale)
            page = context.new_page()
            try:
                page.goto(url, wait_until="load", timeout=45000)
                try:
                    page.wait_for_load_state("networkidle", timeout=8000)
                except PWTimeout:
                    entry["note"] = "network did not go idle within 8s"
                if screen.get("wait_for"):
                    page.wait_for_selector(screen["wait_for"], timeout=15000)
                broken = page.evaluate(READY_JS)
                stable, checks = wait_until_settled(page, 0.4, 2, args.settle_timeout)
                file_name = f"{index:02d}-{safe_name(name)}.png"
                path = out_dir / file_name
                if screen.get("selector"):
                    page.locator(screen["selector"]).first.screenshot(path=str(path))
                else:
                    page.screenshot(path=str(path), full_page=bool(screen.get("full_page")))
                size = page.evaluate("() => [document.documentElement.scrollWidth, document.documentElement.scrollHeight]")
                entry.update({"file": file_name, "url": url, "stable": stable, "layout_checks": checks,
                              "broken_images": broken, "page_size": size, "viewport": vp})
                if size[0] > vp["width"] + 2:
                    entry["warning"] = "page wider than viewport; content may be clipped horizontally"
            except Exception as exc:  # report and continue with the next screen
                entry["error"] = f"{type(exc).__name__}: {exc}"
            finally:
                context.close()
            report.append(entry)
            status = "ok" if entry.get("stable") and not entry.get("broken_images") and "error" not in entry else "CHECK"
            print(f"[{status}] {name} -> {entry.get('file', entry.get('error'))}")
        browser.close()

    (out_dir / "capture-report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    failed = [r for r in report if "error" in r or not r.get("stable") or r.get("broken_images")]
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
