# Install through CLIProxyAPI Plugin Store URL

This repository is prepared so you can upload it to GitHub, create a release, then paste one raw `registry.json` URL into CLIProxyAPI.

## 1. Upload this folder to GitHub

Create a new GitHub repository, for example:

```bash
git init
git add .
git commit -m "add codex-force-websearch plugin store repo"
git branch -M main
git remote add origin git@github.com:YOUR_GITHUB_USER/codex-force-websearch-plugin-store.git
git push -u origin main
```

## 2. Render `registry.json` for your GitHub repo

Run this locally before pushing, or run it in GitHub Codespaces:

```bash
./scripts/render-registry.sh YOUR_GITHUB_USER/codex-force-websearch-plugin-store
git add registry.json
git commit -m "configure plugin store registry"
git push
```

The URL you will paste into CLIProxyAPI is:

```text
https://raw.githubusercontent.com/YOUR_GITHUB_USER/codex-force-websearch-plugin-store/main/registry.json
```

## 3. Build a plugin-store release

Use either method.

### Option A: tag release from your terminal

```bash
git tag v0.1.0
git push origin v0.1.0
```

### Option B: GitHub Actions manual release

Open GitHub → Actions → **Build plugin-store release** → **Run workflow** → keep `v0.1.0` or enter a newer version.

The workflow publishes these release assets:

```text
codex-force-websearch_0.1.0_linux_amd64.zip
codex-force-websearch_0.1.0_linux_arm64.zip
checksums.txt
```

Each zip contains `codex-force-websearch.so` at the zip root, which is the format required by CLIProxyAPI's plugin store.

## 4. Add the registry URL to CLIProxyAPI

In CLIProxyAPI config:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  store-sources:
    - "https://raw.githubusercontent.com/YOUR_GITHUB_USER/codex-force-websearch-plugin-store/main/registry.json"
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

Restart CLIProxyAPI or hot-reload your config. Then install from the management UI plugin store, or use the management API endpoint if you prefer automation.

## 5. Docker note

If CLIProxyAPI runs in Docker, the plugin store install happens inside the container and selects the container platform automatically. You do not need to manually mount a `.so` if the container has persistent access to its `plugins` directory.

Make sure your Docker volume persists CLIProxyAPI's plugin directory. Example:

```yaml
services:
  cli-proxy-api:
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auth-dir:/root/.cli-proxy-api
      - ./plugins:/CLIProxyAPI/plugins
```

## Verify

Use the management API or UI and confirm:

```text
codex-force-websearch
registered: true
effective_enabled: true
```

## Optional: jshandler script

`jshandler-scripts/codex_force_websearch.js` is included for reference. It can be used with the official `jshandler` plugin if you manually mount the JS file and configure `script_paths`, but the recommended path for "paste registry URL into CLIProxyAPI" is the native plugin-store release in this repo.
