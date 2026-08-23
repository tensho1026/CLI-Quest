# Changelog

## v0.2.0 - 2026-08-23

### Added

- Interactive category, difficulty, and scene selection
- Category/difficulty/status filters and `next`
- `cancel`, `cleanup`, `open`, `solution`, `history`, `stats`, and `completion`
- Clear explanations, alternative solutions, cautions, and Japanese translations
- Difficulty- and hint-aware XP, attempt history, best times, and learning streaks
- Recursive `all` and `any` validators plus reusable file and Git validators
- 13 new scenes, for 24 total across seven categories
- JSON output, persistent language settings, and bash/zsh/fish completion
- Scene JSON Schema and external definition validation
- Progress file locking, stale helper registry cleanup, and symlink escape protection
- macOS/Linux CI, automated release archives, checksums, and Homebrew Formula

### Compatibility

- Existing v0.1 `progress.json` files continue to load. New statistics start accumulating after upgrading.

## v0.1.0 - 2026-08-23

- Initial CLI Quest MVP with 11 Linux, Git, Process, and HTTP scenes.
