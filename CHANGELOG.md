# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial project scaffold: Go backend skeleton, embedded SvelteKit frontend, SQLite
  store with migrations, configuration system and authentication groundwork.
- ICS transform pipeline (`internal/ics`, `internal/pipeline`) over go-ical: parse/
  serialize helpers and sequential rules — filter (black/whitelist), dedup, rename,
  field strip, timezone normalization and expiry — with substring ("DAU") or regex
  matching.
- ICS proxy with caching (`internal/proxy`, `feed_cache` table): TTL-honored upstream
  fetch with ETag revalidation and stale-on-error fallback to the last good copy.

[Unreleased]: https://github.com/Norrodar/TidyDAV/commits/main
