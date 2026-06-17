# codex-grok-force-search for CLIProxyAPI

A plugin-store-ready CLIProxyAPI request interceptor for **Codex/OpenAI Responses** and **Grok/xAI Responses** traffic.

It injects server-side hosted search tools into request bodies:

```json
// Codex / OpenAI Responses
{ "type": "web_search" }
```

```json
// Grok / xAI Responses
{ "type": "web_search" }
{ "type": "x_search" }
```

Default behavior:

- targets Codex requests when CLIProxyAPI reports `source_format` or `to_format` as `codex`, or when the model matches OpenAI/Codex patterns
- targets Grok/xAI requests when CLIProxyAPI reports `xai`, `x-ai`, or `grok`, or when the model matches `grok*`
- appends OpenAI `web_search` to Codex Responses request bodies
- appends xAI `web_search` and `x_search` to Grok Responses request bodies
- sets `tool_choice: "required"`
- enables `parallel_tool_calls`
- sets `max_tool_calls: 6`
- adds OpenAI `include: ["web_search_call.action.sources"]` for Codex/OpenAI
- appends model-specific instructions asking the model to search current/primary sources

> Important: the plugin can force the tools to be present in the payload and set `tool_choice: "required"`. Server-side model/tool planners still decide exact tool-call sequencing. The Grok instruction explicitly asks for both `web_search` and `x_search` when available.

## Fast path: upload to GitHub, then paste URL into CLIProxyAPI

1. Upload this folder to your GitHub repo: `ewehiuw3743283478/sandbox`.
2. `registry.json` is already rendered for your repo. If you ever move it, rerun:

```bash
./scripts/render-registry.sh OWNER/REPO
git add registry.json
git commit -m "configure plugin registry"
git push
```

3. Create a release by pushing a tag:

```bash
git tag v0.2.0
git push origin v0.2.0
```

4. Add this URL to `plugins.store-sources` in CLIProxyAPI:

```text
https://raw.githubusercontent.com/ewehiuw3743283478/sandbox/main/registry.json
```

See [`PLUGIN_STORE_INSTALL.md`](PLUGIN_STORE_INSTALL.md) for the full Docker/plugin-store flow.

## CLIProxyAPI config

```yaml
plugins:
  enabled: true
  dir: "plugins"
  store-sources:
    - "https://raw.githubusercontent.com/ewehiuw3743283478/sandbox/main/registry.json"
  configs:
    codex-grok-force-search:
      enabled: true
      priority: 50
      enable_codex: true
      enable_grok: true
      inject_before_auth: false
      inject_after_auth: true
      tool_choice_required: true
      set_parallel_tool_calls: true
      max_tool_calls: 6
      search_context_size: "medium"
      include_action_sources: true
      add_instruction: true
```

## Optional Grok filters

```yaml
plugins:
  configs:
    codex-grok-force-search:
      grok_allowed_domains: ["x.ai", "docs.x.ai"]
      grok_allowed_x_handles: ["xai", "grok"]
      grok_from_date: "2026-01-01"
      grok_enable_image_understanding: true
      grok_x_enable_video_understanding: true
```

## Files

```text
go/                                  Native plugin source
.github/workflows/release.yml         Builds plugin-store release assets
registry.json                         Plugin-store registry; render with scripts/render-registry.sh
config.example.yaml                   Full config options
PLUGIN_STORE_INSTALL.md               Upload/release/install steps
DOCKER.md                             Manual Docker mount fallback
jshandler-scripts/force_codex_grok_search.js  Optional JS reference for cpa-plugin-jshandler
codex-plugin/                         Optional Codex-native skill; not required by CLIProxyAPI
```

## Build locally, only if you are not using the plugin store

```bash
make build
```

For Docker CLIProxyAPI, prefer the GitHub Actions release path so the plugin is built for the container platform: `linux/amd64` or `linux/arm64`.


## Your prepared registry URL

Use this exact URL in CLIProxyAPI, assuming your default branch is `main`:

```text
https://raw.githubusercontent.com/ewehiuw3743283478/sandbox/main/registry.json
```

Your URL `https://raw.githubusercontent.com/ewehiuw3743283478/sandbox` is the raw host/repo prefix; CLIProxyAPI needs the full raw file URL including branch and file path.
