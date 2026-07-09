# Contributing

## Git commits — no Cursor Agent attribution

This repository should **not** list [cursoragent](https://github.com/cursoragent) (Cursor Agent) on the GitHub **Contributors** sidebar. All commits must be authored as **you**, not the IDE.

### Do not commit `.cursor/rules/` to git

GitHub links tracked `.cursor/rules/*.mdc` files to the **cursoragent** account on the repo homepage, even when commit authors are correct. Cursor rules belong **only on your machine**:

```bash
./scripts/setup-git-hooks.sh   # installs hooks + copies rule locally
```

Template (copy locally, not in git): [`docs/agent-rules/no-cursor-agent-contributor.mdc`](agent-rules/no-cursor-agent-contributor.mdc)

### Disable in Cursor (recommended)

1. Open **Cursor Settings**
2. Go to **Agents → Attribution** (or **Git & PRs → Attribution**)
3. Turn **off** Commit Attribution and PR Attribution
4. Restart Cursor

For CLI/cloud agents, set in `~/.cursor/cli-config.json`:

```json
{
  "commitAttribution": false,
  "prAttribution": false
}
```

### Repo git hook (extra safety)

After cloning, run once:

```bash
./scripts/setup-git-hooks.sh
```

This sets `core.hooksPath` to `.githooks/`, which removes `Co-authored-by: Cursor …` and `Made-with: Cursor` lines before each commit is created.

### Removing cursoragent from GitHub

There is no “remove contributor” button. GitHub derives the sidebar from commits and from **`.cursor/rules/` in the default branch**.

1. **Stop tracking** `.cursor/rules/` (this repo ignores `.cursor/`).
2. Wait for GitHub to refresh the contributors list (can take hours).
3. If `cursoragent` still appears, check for old `Co-authored-by: Cursor` trailers:
   ```bash
   git log --all --format=%B | rg -i 'co-authored-by:.*cursor|cursoragent@|made-with:.*cursor' || echo "clean"
   ```
4. Only if trailers exist: rewrite history with `git filter-repo` and force-push (changes SHAs/tags).

### `github-actions[bot]`

You may also see **github-actions[bot]** as a contributor (two legacy commits that committed release binaries). That is **not** Cursor Agent. The current release workflow only uploads assets to GitHub Releases and does **not** push commits — removing the bot from history is optional and **does not** affect today’s release workflow.
