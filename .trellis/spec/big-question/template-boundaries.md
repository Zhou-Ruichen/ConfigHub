# Template Boundary Guide

## Template Acceptance Test

A template is acceptable only if:

- its source is intentional and reviewable;
- its target path is explicit;
- it does not include runtime state by accident;
- its secret behavior is declared (`safety.secrets: allowed | forbidden`);
- its merge strategy is declared (`merge: replace | managed-section | deep-merge`);
- rollback is possible for sync delivery.

## Dotfiles Templates

Good early candidates:

- Git config fragments (via `~/.config/confighub/fragments/dotfiles/git/...` + `managed-section` include in `~/.gitconfig`).
- Shell fragments (Zsh first).
- Editor settings with clear ownership.
- Terminal settings with clear ownership.
- Shared helper scripts.

Bad candidates:

- Private SSH keys.
- `known_hosts`.
- Shell history.
- App databases.
- Caches.
- Logs.
- GUI state.
- Generated session files.

Symlink-based dotfiles managers (stow, chezmoi) require special handling: see "Symlink-managed dotfiles" in [../shared/config-safety.md](../shared/config-safety.md).

## AI Templates

Good early candidates:

- Model/provider endpoint settings.
- MCP endpoint settings.
- Tool settings JSON/TOML that are not session databases.

These almost always carry secret-derived values (API keys, OAuth tokens). They must declare `safety.secrets: allowed` and follow the full secret-handling flow in [./secret-handling.md](./secret-handling.md).

Bad candidates:

- Login state.
- OAuth refresh tokens that the tool manages internally.
- History files.
- Local tool databases.
- Cache directories.

## Merge Strategy Per Domain

- AI tools that maintain rich user-side customization (`~/.claude/settings.json` with hooks, `~/.codex/config.toml` with custom commands): prefer `merge: managed-section` so user edits survive apply.
- Endpoint-only files (provider profiles, MCP endpoints) with no user-side customization: `merge: replace` is acceptable.
- Dotfiles include lines: always `merge: managed-section` with `includeStrategy: append-once`.

## When This Guide Triggers a Review

- A new template is added to any domain.
- A target path is changed.
- A merge strategy changes for an existing template.
- A new domain is proposed.
