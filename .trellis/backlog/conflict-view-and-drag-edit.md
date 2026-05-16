# Backlog: Conflict View and Drag-Edit UI

Status: deferred (post-v1). Captured 2026-05-16.

## The Idea

Add a Web UI that:

1. Shows, per profile, every config item with a visual marker for "shared" vs "local".
2. Detects conflicts between the hub's rendered bundle and the client device's current files, and surfaces them as a notification.
3. Lets the operator drag an item between "shared" and "local" (or between profiles) to change scope.
4. Lets the operator drag-edit individual items (e.g. one SSH host alias) and have the change propagate to the hub's profile/template files.

## What's Genuinely Valuable

- **Conflict view**: visual diff between hub-rendered bundle and client-side actual state. Today `confighub diff` produces text only; a per-entry view with red/green markers is a real UX win.
- **Per-item scope visibility**: making "this lives in everyone's bundle vs only this machine's" inspectable encourages the operator to decide explicitly rather than have everything default to one bucket.
- **Set diff between profiles**: "this alias is on macbook but missing on mini" is concrete value that text diff hides.

## Why the Write-Path Half Is Risky

- **Breaks single-writer assumption** documented in `.trellis/spec/shared/concurrency.md`. The hub becomes a second writer alongside the operator's text editor / git workflow. Locking, audit log, and history become required.
- **No commit message, no PR**: drag-edit writes lose the "why" that text + git provide. For an ops tool this is a regression, not progress.
- **Fights GitOps**: if the operator keeps `profiles/` and `templates/` in a git repo (recommended by README), UI writes create a parallel write path that has to be reconciled with git pulls. This is the same trap that killed many "config UI on top of git" attempts.
- **Heavy JS**: drag-and-drop needs at minimum HTMX + Sortable.js, more likely a SPA framework. Reverses the "minimal JS" decision in `spec/frontend/index.md`.
- **Per-domain parsers**: to make `~/.ssh/config` items individually draggable, ConfigHub needs an SSH config AST. Same for every dotfile format. This is N parsers, where N grows with every domain.

## Modernity Note

"Drag-and-drop config editor" is a 2010s SaaS pattern. 2026 ops tooling has converged on GitOps (Argo CD, Flux, Atlantis) and version-explicit secret tools (Vault, Doppler, SOPS) where the UI is read-mostly and writes are explicit, versioned, and reviewable. ConfigHub leaning toward the former is on-trend; toward the latter would be the regressive direction.

## Recommended Slice Split

- **Slice 4.5 (or part of Slice 4)** — conflict view, read-only:
  - Bundle detail page lists each manifest entry alongside the client's reported state (when the client posts a status report or pulls).
  - Mark mismatches; show which entry is hub-newer vs locally-modified.
  - "Promote local change back to hub" is a copy-to-clipboard CLI command, not a write button.
  - Fits the current single-writer model. Minimal JS.

- **Deferred to v2 (post-Slice 8)** — drag-edit write path:
  - Reopen this only if (a) the operator has measurable pain from the text + git workflow, and (b) there is a real audit + locking design.
  - If reopened, write a proper RFC covering: writer lock, conflict resolution between UI and git, change history, undo, per-action commit message UX.

## When to Reconsider

- After Slice 8 (MVP complete and dogfooded on real machines).
- Triggered by concrete operator pain: edit cycles per week, mistakes from text editing, missed shared-vs-local distinctions.
- Not triggered by "the UI would be cooler".

## Related

- See [local-differentiation.md](./local-differentiation.md) for the underlying data-model question: should "this device differs from shared" become a first-class concept (independent of UI)? That decision should land before this one — UI is a presentation layer over whatever the data model becomes.
