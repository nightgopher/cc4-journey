#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$ROOT/ui-library-selector/SKILL.md"
TESTS="$ROOT/tests/test-prompts.json"
ARCHIVE="$ROOT/ui-library-selector-skill.tar.gz"
ERRORS=0

pass() {
  printf 'PASS %s\n' "$1"
}

fail() {
  printf 'FAIL %s\n' "$1" >&2
  ERRORS=$((ERRORS + 1))
}

require_file() {
  if [[ -f "$1" ]]; then
    pass "${1#"$ROOT/"} exists"
  else
    fail "${1#"$ROOT/"} is missing"
  fi
}

require_pattern() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  if grep -qE "$pattern" "$file"; then
    pass "$label"
  else
    fail "$label"
  fi
}

scan_tree() {
  local pattern="$1"
  local path="$2"
  if command -v rg >/dev/null 2>&1; then
    rg -n "$pattern" "$path"
  else
    grep -R -n -E "$pattern" "$path"
  fi
}

check_archive_file() {
  local relative_path="$1"
  if tar -xOf "$ARCHIVE" "$relative_path" 2>/dev/null | cmp -s - "$ROOT/$relative_path"; then
    pass "archive matches $relative_path"
  else
    fail "archive is stale or missing $relative_path"
  fi
}

require_file "$SKILL"
require_file "$ROOT/ui-library-selector/references/decision-matrix.md"
require_file "$ROOT/ui-library-selector/references/component-search-index.md"
require_file "$ROOT/ui-library-selector/references/install-commands.md"
require_file "$ROOT/ui-library-selector/references/library-catalog.md"
require_file "$TESTS"
require_file "$ARCHIVE"

if [[ -f "$SKILL" ]]; then
  require_pattern "$SKILL" '^name: ui-library-selector$' "frontmatter name is valid"
  require_pattern "$SKILL" '^description: >-' "frontmatter has a rich description"
  require_pattern "$SKILL" 'UI美化|页面不好看|改布局' "colloquial Chinese triggers are present"
  require_pattern "$SKILL" 'Do not trigger for a mere UI mention' "negative trigger is present"
  require_pattern "$SKILL" 'any point in a conversation' "mid-conversation activation is explicit"
  require_pattern "$SKILL" 'official shadcn Skill or shadcn MCP' "library-specific handoff is explicit"
  require_pattern "$SKILL" '^## Product Surface Gate$' "multi-surface decomposition gate is explicit"
  require_pattern "$SKILL" 'weights totaling 100%' "surface-specific objective weights are required"
  require_pattern "$SKILL" 'behavior foundation.*visual effect source.*motion orchestration' "three-layer selection contract is explicit"
  require_pattern "$SKILL" '^## Effect Source Gate$' "explicit wow requests have an effect-source gate"
  require_pattern "$SKILL" '^## Rendered Evidence Gate$' "rendered visual evidence gate is explicit"
  require_pattern "$SKILL" 'search.*,.*view.*,.*add --dry-run' "registry search-view-dry-run chain is explicit"
  require_pattern "$ROOT/ui-library-selector/references/decision-matrix.md" 'Threads.*LiquidEther.*Dither' "multi-surface regression shortlist is documented"
  require_pattern "$ROOT/ui-library-selector/references/install-commands.md" 'Motion for React.*formerly Framer Motion' "motion orchestration source is documented"

  LINE_COUNT="$(wc -l < "$SKILL" | tr -d ' ')"
  if [[ "$LINE_COUNT" -le 500 ]]; then
    pass "SKILL.md stays under 500 lines ($LINE_COUNT)"
  else
    fail "SKILL.md exceeds 500 lines ($LINE_COUNT)"
  fi
fi

if scan_tree 'https://reactbits\.dev/r/|https://magicui\.design/r/' "$ROOT/ui-library-selector" >/dev/null 2>&1; then
  fail "deprecated raw registry URL patterns remain"
else
  pass "deprecated raw registry URL patterns are absent"
fi

if scan_tree '/Users/|/home/|C:\\Users\\' "$ROOT/ui-library-selector" >/dev/null 2>&1; then
  fail "private absolute paths found in distributable Skill"
else
  pass "no private absolute paths in distributable Skill"
fi

if [[ -f "$ARCHIVE" ]]; then
  check_archive_file "ui-library-selector/SKILL.md"
  check_archive_file "ui-library-selector/references/decision-matrix.md"
  check_archive_file "ui-library-selector/references/component-search-index.md"
  check_archive_file "ui-library-selector/references/install-commands.md"
  check_archive_file "ui-library-selector/references/library-catalog.md"
fi

if command -v node >/dev/null 2>&1; then
  if node -e 'const f=require(process.argv[1]); if(f.version<2||!Array.isArray(f.cases)||f.cases.length<10) process.exit(1); for(const c of f.cases){if(!c.id||!c.prompt||typeof c.expected?.trigger!=="boolean") process.exit(1)} const ids=new Map(f.cases.map(c=>[c.id,c])); const m=ids.get("multi-surface-portfolio-admin"); const w=ids.get("explicit-wow-effect-gate"); if(!m||m.expected.surface_count!==2||!w) process.exit(1); const must=m.expected.must||[]; if(!must.some(x=>x.includes("behavior foundation"))||!must.some(x=>x.includes("Threads"))) process.exit(1)' "$TESTS"; then
    pass "test-prompts.json is valid and contains multi-surface regression contracts"
  else
    fail "test-prompts.json schema check failed"
  fi

  if node -e 'const fs=require("fs"); const s=fs.readFileSync(process.argv[1],"utf8"); const m=s.match(/^description: >-\n([\s\S]*?)\n---/m); if(!m) process.exit(1); const d=m[1].split(/\r?\n/).map(x=>x.trim()).join(" "); if(d.length>1024) process.exit(1)' "$SKILL"; then
    pass "frontmatter description stays within 1024 characters"
  else
    fail "frontmatter description exceeds 1024 characters"
  fi
else
  fail "node is required to validate test-prompts.json"
fi

if [[ "$ERRORS" -gt 0 ]]; then
  printf '\n%d check(s) failed.\n' "$ERRORS" >&2
  exit 1
fi

printf '\nAll checks passed.\n'
