# Configuration Safety

## Forbidden By Default

ConfigHub must not manage:

- private keys;
- login sessions;
- browser cookies;
- OAuth refresh tokens that the tool manages internally;
- CLI histories;
- local runtime databases;
- caches;
- logs;
- GUI state;
- `known_hosts`;
- arbitrary home-directory trees.

## Target Path Policy

Every template must declare its target path and safety policy.

Default behavior:

- reject unknown target roots (only `~`, `~/.config/...`, `~/.confighub/...`, and explicitly allowlisted dotfile paths are accepted);
- reject symlinks (templates may opt in with `safety.symlink: replace | follow`);
- require diff;
- require backup;
- require manifest membership;
- require checksum verification.

Allowlisted bare dotfiles at `$HOME` (paths exactly equal to `$HOME/<name>`):

- `.gitconfig`, `.gitconfig.local`
- `.zshrc`, `.zshrc.local`
- `.zshenv`, `.zshenv.local`
- `.zprofile`, `.zprofile.local`
- `.bashrc`, `.bashrc.local`
- `.bash_profile`, `.bash_profile.local`
- `.profile`
- `.tmux.conf`
- `.vimrc`

The paired `.local` variants exist because ConfigHub renders per-device override files that the operator's shared dotfiles (e.g. `~/.gitconfig` `[include]` directive, `~/.zshrc` `source ~/.zshrc.local`) reference.

### Symlink-managed Dotfiles (stow / chezmoi)

ConfigHub's default symlink rejection means it does not drop-in compose with stow or chezmoi-managed symlinks. Two supported interop options:

1. Let ConfigHub render fragments under `~/.config/confighub/fragments/...` and let stow/chezmoi manage the host file's include line.
2. Hand ownership of the host file to ConfigHub (use `merge: replace` or `merge: managed-section`) and stop tracking it in stow/chezmoi.

The README must call this out before users adopt the dotfiles domain.

## Settings Merge Strategy

Tool settings files often mix ConfigHub-managed content with user-owned content. Each template declares one of:

- **`merge: replace`** — ConfigHub owns the entire file. User edits are clobbered. The template description and README must make this explicit.
- **`merge: managed-section`** — ConfigHub writes its content inside a marker block (`# >>> confighub:<name> >>>` ... `# <<< confighub:<name> <<<` or the file format's comment equivalent). User content above and below the markers is preserved. On apply, only the marker block is rewritten. On rollback, the marker block is restored from the backup.
- **`merge: deep-merge`** — *Reserved for a later slice.* When implemented, ConfigHub deep-merges its rendered keys under a `confighub.*` namespace; keys outside that namespace are preserved.

MVP supports `replace` and `managed-section`. Templates that need `deep-merge` must wait for the format-specific merge spec.

### Marker Block Format

| File format | Open marker | Close marker |
| --- | --- | --- |
| Shell (`.zshrc`, `.bashrc`) | `# >>> confighub:<name> >>>` | `# <<< confighub:<name> <<<` |
| Git config | `# >>> confighub:<name> >>>` | `# <<< confighub:<name> <<<` |
| TOML | `# >>> confighub:<name> >>>` | `# <<< confighub:<name> <<<` |
| JSON | not supported (use `merge: replace` or `deep-merge` once available) |

Marker names use kebab-case and embed the template id.

### `includeStrategy: append-once`

For templates whose job is to write an `include` line into a host file (e.g. `[include] path = ~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig` into `~/.gitconfig`):

- The apply engine searches for the matching marker block.
- If absent, it appends the marker block + include content to the file.
- If present, it leaves the file unchanged.
- Subsequent applies are idempotent.
- On rollback, the marker block is removed and any pre-existing content above/below the marker block is preserved exactly.

## Secret Handling

Templates should prefer references to secrets over raw secret material.

Rendered files may contain secret-derived values only when:

- the template explicitly allows it (`safety.secrets: allowed`);
- the profile is authorized;
- logs and apply metadata redact the value;
- the destination file is expected to contain that value.

See [../big-question/secret-handling.md](../big-question/secret-handling.md) for the full secret-handling flow.

## Apply Safety

Every local apply must:

1. Verify manifest and checksums.
2. Show a diff unless `--yes` is provided.
3. Back up existing files (under `~/.confighub/backups/<timestamp>/`, mode `0700`/`0600`).
4. Write through a temp file in the target directory.
5. Atomically rename into place.
6. Record an apply log.
7. Provide rollback.
8. Process `removedFiles` (back up then delete; refuse if local checksum does not match `previousChecksum`).

Hub trust is not enough. A pulled bundle still goes through local verification (manifest + checksums + signature from Slice 5).
