# codex-force-websearch for CLIProxyAPI

A plugin-store-ready CLIProxyAPI request interceptor for Codex / OpenAI Responses API traffic.

It injects OpenAI's hosted server-side web search tool into Codex-shaped requests:

```json
{ "type": "web_search" }
```

Default behavior:

- only targets requests where CLIProxyAPI reports `source_format` or `to_format` as `codex`
- appends a `web_search` tool to Responses API request bodies
- sets `tool_choice: "required"`
- enables `parallel_tool_calls`
- sets `max_tool_calls: 4`
- adds `include: ["web_search_call.action.sources"]`
- appends an instruction asking Codex to search primary/current sources

## Fast path: upload to GitHub, then paste URL into CLIProxyAPI

1. Upload this folder to a GitHub repo.
2. Run:

```bash
./scripts/render-registry.sh YOUR_GITHUB_USER/YOUR_REPO
git add registry.json
git commit -m "configure plugin registry"
git push
```

3. Create a release by pushing a tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

4. Add this URL to `plugins.store-sources` in CLIProxyAPI:

```text
https://raw.githubusercontent.com/YOUR_GITHUB_USER/YOUR_REPO/main/registry.json
```

See [`PLUGIN_STORE_INSTALL.md`](PLUGIN_STORE_INSTALL.md) for the full Docker/plugin-store flow.

## CLIProxyAPI config

```yaml
plugins:
  enabled: true
  dir: "plugins"
  store-sources:
    - "https://raw.githubusercontent.com/YOUR_GITHUB_USER/YOUR_REPO/main/registry.json"
  configs:
    codex-force-websearch:
      enabled: true
      priority: 50
      require_codex_format: true
      target_formats: ["codex"]
      inject_before_auth: false
      inject_after_auth: true
      tool_choice_required: true
      set_parallel_tool_calls: true
      max_tool_calls: 4
      search_context_size: "medium"
      include_action_sources: true
      add_instruction: true
```

## Files

```text
go/                                  Native plugin source
.github/workflows/release.yml         Builds plugin-store release assets
registry.json                         Plugin-store registry; render with scripts/render-registry.sh
config.example.yaml                   Full config options
PLUGIN_STORE_INSTALL.md               Upload/release/install steps
DOCKER.md                             Manual Docker mount fallback
jshandler-scripts/                    Optional JS reference for cpa-plugin-jshandler users
```

## Important limitation

This forces the OpenAI Responses API `web_search` tool into the request. It does not grant internet access to a separate Codex cloud sandbox or shell runtime.
