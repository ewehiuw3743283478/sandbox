# Upload this prepared repo to GitHub

This package is already configured for:

```text
ewehiuw3743283478/sandbox
```

Use these commands from the extracted folder:

```bash
git init
git add .
git commit -m "add codex grok force search plugin"
git branch -M main
git remote add origin git@github.com:ewehiuw3743283478/sandbox.git
git push -u origin main

git tag v0.2.0
git push origin v0.2.0
```

After GitHub Actions finishes the release, paste this exact registry URL into CLIProxyAPI:

```text
https://raw.githubusercontent.com/ewehiuw3743283478/sandbox/main/registry.json
```

If your GitHub repo default branch is not `main`, replace `main` in that URL with your real branch name and update `registry.json` on that branch.
