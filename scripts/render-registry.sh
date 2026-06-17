#!/usr/bin/env bash
set -euo pipefail

repo="${1:-}"
if [[ -z "$repo" ]]; then
  if git remote get-url origin >/dev/null 2>&1; then
    origin="$(git remote get-url origin)"
    repo="$(printf '%s' "$origin" | sed -E 's#^git@github.com:##; s#^https://github.com/##; s#\.git$##')"
  fi
fi

if [[ -z "$repo" || "$repo" != */* ]]; then
  echo "Usage: $0 OWNER/REPO" >&2
  exit 2
fi

owner="${repo%%/*}"
repository_url="https://github.com/${repo}"
python3 - <<PY
import json
from pathlib import Path
path = Path('registry.json')
data = json.loads(path.read_text())
plugin = data['plugins'][0]
plugin['author'] = ${owner@Q}
plugin['repository'] = ${repository_url@Q}
plugin['homepage'] = ${repository_url@Q}
path.write_text(json.dumps(data, indent=2) + '\n')
PY

echo "Updated registry.json for ${repository_url}"
echo "Store-source URL will be: https://raw.githubusercontent.com/${repo}/main/registry.json"
