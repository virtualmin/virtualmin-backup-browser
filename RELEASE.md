## Releasing

Releases are automated with [GoReleaser](https://goreleaser.com). Pushing a tag
cross-builds every target, publishes a GitHub release, and updates the Homebrew
tap and Scoop bucket:

```
git tag v0.1.0
git push origin v0.1.0
```

This requires a `TAP_GITHUB_TOKEN` repository secret: a token with write access
to `virtualmin/homebrew-tap` and `virtualmin/scoop-bucket` (fine-grained with
contents:write on both repos, or a classic token with `repo` scope). Validate
config changes locally with `goreleaser check`.

