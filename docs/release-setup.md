# Release Setup Action Items

## GitHub Repository Secrets

Configure the following secrets in `Settings > Secrets and variables > Actions`:

- [ ] **`HOMEBREW_TAP_GITHUB_TOKEN`** — A personal access token (PAT) with `repo` scope, used by GoReleaser to push the Homebrew formula to `sha1n/homebrew-tap`. Create one at https://github.com/settings/tokens.

> `GITHUB_TOKEN` is provided automatically by GitHub Actions.

## Homebrew Tap Repository

- [ ] Ensure the `sha1n/homebrew-tap` repository exists on GitHub
- [ ] Ensure the `Formula/` directory exists in that repository (GoReleaser will create the formula file automatically on release)

## Branch Protection (Optional)

- [ ] Set `status-check` as a required status check on the `master` branch to gate merges on CI passing

## Release Drafter

- [ ] The release drafter is configured and will run automatically on pushes to `master` and PR events
- [ ] PR titles following conventional commit format (`feat:`, `fix:`, `docs:`, etc.) will be auto-labeled

## Creating a Release

To create a release:

1. Tag the commit: `git tag v0.1.0`
2. Push the tag: `git push origin v0.1.0`
3. The `release.yml` workflow will trigger automatically, running GoReleaser to:
   - Build darwin/amd64 and darwin/arm64 binaries
   - Create a GitHub release with checksums and archives
   - Push the Homebrew formula to `sha1n/homebrew-tap`

Alternatively, run locally (requires `GITHUB_TOKEN` and `HOMEBREW_TAP_GITHUB_TOKEN` env vars):

```bash
make release
```

## Installing via Homebrew (After First Release)

```bash
brew tap sha1n/tap
brew install projmark
```
