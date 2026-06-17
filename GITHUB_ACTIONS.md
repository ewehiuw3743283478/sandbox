# GitHub Actions release build

This repo includes `.github/workflows/release.yml`.

It builds plugin-store-compatible release assets for:

```text
linux/amd64
linux/arm64
```

## Run by tag

```bash
git tag v0.2.0
git push origin v0.2.0
```

## Run manually

GitHub → Actions → **Build plugin-store release** → **Run workflow** → enter `v0.2.0`.

The release will contain:

```text
codex-grok-force-search_0.2.0_linux_amd64.zip
codex-grok-force-search_0.2.0_linux_arm64.zip
checksums.txt
```

Each zip contains the shared library at the archive root:

```text
codex-grok-force-search.so
```

That is the asset shape CLIProxyAPI's plugin store expects.
