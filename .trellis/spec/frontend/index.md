# Web UI Guidelines

ConfigHub's first UI is server-rendered and embedded in the Go binary.

## Documentation Files

| File | Description | When to Read |
| --- | --- | --- |
| [admin-ui.md](./admin-ui.md) | Page structure and UI behavior | Any web UI work |
| [quality.md](./quality.md) | Accessibility and interaction checks | Before UI completion |

## Core Rules

- The UI supports the "open and use" workflow.
- Pages work without mandatory client-side JavaScript.
- Minimal embedded JavaScript is allowed for copy-to-clipboard and diff toggle only.
- Keep the first UI operational and dense: profiles, templates, bundles, bootstrap commands, warnings.
- Do not add a SPA framework until server-rendered pages fail a concrete requirement.
- Every dangerous action is explicit and inspectable.

## MVP Pages

- Home/status.
- Profile list.
- Profile detail.
- Template/domain list.
- Bundle detail with manifest, checksums, `removedFiles`.
- Bootstrap command page.
- Warning/detail page for unsafe or disallowed templates.
