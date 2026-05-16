# Backlog: Local Differentiation (Shared vs Local Layering)

Status: under consideration. Captured 2026-05-16.

## The Question

Should ConfigHub support "this device differs from shared" as a first-class concept, separate from creating an entirely new profile per device?

Current model (v1):

- A **profile** (e.g. `macbook`) selects a set of templates.
- Each template renders to one output.
- Two devices that need different output for the same template require two profiles, even if the only difference is a single key.

Problem this causes once an operator owns >2 machines:

- Profile explosion: `macbook`, `mini`, `vps-1`, `vps-2` — most fields are identical, the differences are small.
- Drift risk: a shared change has to be hand-applied to every profile file.
- Cognitive overhead: "which profile holds the canonical SSH config" is unclear.

## Two Honest Options

### Option A: Hub-Side Layered Profiles (recommended starting point)

A profile can declare `extends: <parent-profile>`. The render engine merges parent + child before rendering.

```yaml
# profiles/_shared.yaml
id: _shared
domains: { ai: true, dotfiles: true }
allowedTemplates: [ai/claude, dotfiles/ssh, dotfiles/git]

# profiles/macbook.yaml
id: macbook
extends: _shared
allowedTemplates+: [dotfiles/work-aliases]   # additive
templateOverrides:
  dotfiles/ssh:
    vars:
      hostname_suffix: ".local"
```

Pros:
- Single source of truth stays on the hub.
- Compatible with GitOps: git history shows who changed what, when.
- Single-writer assumption preserved.
- No client-side change.

Cons:
- Merge semantics need spec (deep merge? list append? key replace?). One more design surface.
- Renderer becomes a two-step process: resolve hierarchy, then render.

### Option B: Client-Side Overlays

A per-device file at `~/.config/confighub/overlay/<profile>/<template>.patch` overrides the bundle's rendered output on apply.

Pros:
- Operator can tweak a device without redeploying the bundle.
- "Local" is literally local — no hub round-trip.

Cons:
- Adds a second writer surface (the bundle + the overlay) that conflict detection has to reason about.
- Apply log needs to track "base + overlay = final" rather than "rendered = final".
- Drift is now legitimate (overlay applied) AND illegitimate (manual edit) — distinguishing them is hard.
- Breaks "bundle is the unit of truth" simplicity.

### Option C: Do Nothing (status quo)

Operator creates one profile per device. Differences live in profile files maintained on the hub.

Pros:
- Already supported. No new spec.
- Forces explicit per-device thinking.

Cons:
- Profile duplication and drift become real once N > 2.

## Recommendation

**Plan for Option A (hub-side layered profiles), but not in v1 unless the operator hits the pain.**

Concretely:

- v1 (Slices 1-8): ship Option C (single-profile-per-device).
- Before any second profile is added in real use, decide whether to introduce Option A.
- Option B stays deferred to post-MVP unless a use case appears that Option A genuinely cannot solve.

## Impact on Current Slices If We Adopt Option A

| Slice | Impact |
| --- | --- |
| Slice 2 (render) | Renderer must resolve `extends:` before rendering. Adds ~15-20% to Slice 2 scope. |
| Slice 3 (apply) | No change — apply still sees one final bundle. |
| Slice 4 (serve) | UI must show "from parent" badges on inherited fields. Minor. |
| Bundle contract | `sourceProfiles: ["_shared", "macbook"]` field added to manifest for traceability. Backwards-compatible. |

If we want Option A in v1, the decision must land before Slice 2 starts (the render engine is built once; retrofitting layering later is painful).

## Open Sub-Questions

- Merge semantics for lists vs maps in profiles (e.g. `allowedTemplates`: append or replace?).
- Whether parent profiles are first-class (have their own bundles?) or abstract-only (cannot be rendered standalone).
- Naming: `extends:`, `inherits:`, `bases:` (Kustomize-style)?

## When to Reconsider

- Before Slice 2 dispatch (now): is this in v1?
- After Slice 4 (when UI surfaces multiple profiles and duplication becomes visible): is the pain real?
- Whenever the operator adds a third or fourth profile: forced re-think.

## Related

- [conflict-view-and-drag-edit.md](./conflict-view-and-drag-edit.md) — UI presentation layer over whatever this data model becomes.
