#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 OWNER/REPO" >&2
  exit 2
fi

REPO="$1"
RAW="https://raw.githubusercontent.com/${REPO}/main/registry.json"
GITHUB="https://github.com/${REPO}"

cat > registry.json <<JSON
{
  "schema_version": 1,
  "plugins": [
    {
      "id": "codex-grok-force-search",
      "name": "Codex + Grok Force Search",
      "description": "CLIProxyAPI request interceptor that injects OpenAI web_search for Codex/OpenAI Responses and xAI web_search + x_search for Grok/xAI Responses.",
      "author": "local",
      "version": "0.2.0",
      "repository": "${GITHUB}",
      "homepage": "${GITHUB}",
      "license": "MIT",
      "tags": ["request-interceptor", "codex", "openai", "grok", "xai", "web-search", "x-search"]
    }
  ]
}
JSON

cat <<MSG
Updated registry.json.

Add this to CLIProxyAPI plugins.store-sources:
${RAW}
MSG
