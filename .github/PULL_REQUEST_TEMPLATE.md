## Summary

<!-- What does this PR change and why? Link any related issues (e.g. Closes #123). -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor / chore
- [ ] Documentation

## Checklist

- [ ] **I have read and agree to the [Contributor License Agreement](../CLA.md).**
      Signing the CLA is mandatory before this PR can be merged.
- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
      and are written in English.
- [ ] Added or updated **unit tests** for the changed behavior.
- [ ] `go test ./...`, `go vet ./...` and `golangci-lint run` pass locally.
- [ ] Frontend (if touched): `npm run check` and `npm run build` pass.
- [ ] No new CGO dependency — the build stays `CGO_ENABLED=0`.
- [ ] No hardcoded secrets or config values (everything via `TIDYDAV_*`).
- [ ] Updated `CHANGELOG.md` under **Unreleased**.

## Notes for reviewers

<!-- Anything reviewers should pay special attention to, screenshots for UI changes, etc. -->
