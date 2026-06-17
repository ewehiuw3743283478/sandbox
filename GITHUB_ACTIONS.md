# GitHub Actions release workflow

This repo uses `.github/workflows/release.yml` to build CLIProxyAPI plugin-store release assets.

## What the workflow outputs

For Docker/CLIProxyAPI Linux installs, it publishes:

```text
codex-force-websearch_<version>_linux_amd64.zip
codex-force-websearch_<version>_linux_arm64.zip
checksums.txt
```

Each zip contains `codex-force-websearch.so` at the zip root.

## Run it

Push a tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Or use GitHub UI:

Actions → **Build plugin-store release** → **Run workflow** → enter `v0.1.0`.

Then add your raw `registry.json` URL to CLIProxyAPI `plugins.store-sources`.
