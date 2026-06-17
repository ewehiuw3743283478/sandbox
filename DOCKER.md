# Docker deployment for `codex-grok-force-search`

Preferred path: use the GitHub/plugin-store release flow in `PLUGIN_STORE_INSTALL.md`. CLIProxyAPI inside Docker will download the correct Linux plugin asset for the container architecture.

Manual fallback: build the native shared library for the **container platform** and mount it into `/CLIProxyAPI/plugins`.

## 1. Build the Linux plugin with Docker

From this plugin directory:

```bash
# For most x86_64 servers
mkdir -p plugins/linux/amd64
docker run --rm \
  -v "$PWD":/src \
  -w /src/go \
  --platform linux/amd64 \
  golang:1.26-bookworm \
  bash -lc 'go mod download && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildmode=c-shared -o ../plugins/linux/amd64/codex-grok-force-search.so .'
```

For ARM64 servers, such as Apple Silicon or ARM VPS:

```bash
mkdir -p plugins/linux/arm64
docker run --rm \
  -v "$PWD":/src \
  -w /src/go \
  --platform linux/arm64 \
  golang:1.26-bookworm \
  bash -lc 'go mod download && CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -buildmode=c-shared -o ../plugins/linux/arm64/codex-grok-force-search.so .'
```

If Docker says the Go image tag does not exist yet, use the same Go version required by your checked-out CLIProxyAPI/plugin module, or build inside the CLIProxyAPI source tree using its build image/script.

## 2. Mount plugins into CLIProxyAPI Docker

### docker run

Add this volume to your existing CLIProxyAPI command:

```bash
-v "$PWD/plugins:/CLIProxyAPI/plugins"
```

Full example:

```bash
docker run --rm \
  -p 8317:8317 \
  -v /path/to/your/config.yaml:/CLIProxyAPI/config.yaml \
  -v /path/to/your/auth-dir:/root/.cli-proxy-api \
  -v "$PWD/plugins:/CLIProxyAPI/plugins" \
  eceasy/cli-proxy-api:latest
```

### docker compose

Add the plugins volume under your `cli-proxy-api` service:

```yaml
services:
  cli-proxy-api:
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auth-dir:/root/.cli-proxy-api
      - ./plugins:/CLIProxyAPI/plugins
```

Then restart:

```bash
docker compose up -d --force-recreate
# or
docker compose restart cli-proxy-api
```

## 3. Enable the plugin in config.yaml

Make sure your CLIProxyAPI config includes:

```yaml
plugins:
  enabled: true
  dir: "plugins"
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

## 4. Verify

```bash
docker compose exec cli-proxy-api sh -lc 'find /CLIProxyAPI/plugins -maxdepth 3 -type f -print && uname -m'
docker compose logs -f cli-proxy-api
```

The important rule: do not copy a macOS `.dylib` into Docker. Linux containers need `.so`, built for `linux/amd64` or `linux/arm64`.
