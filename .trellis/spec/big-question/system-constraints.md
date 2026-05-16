# System Constraints

## MVP Constraints

- One Go binary.
- File-backed storage.
- Local, LAN, and VPS deployment with the same artifact.
- No mandatory database.
- No mandatory external secret store.
- No mandatory provider gateway.
- No mandatory MCP gateway.
- Server-rendered UI first.

## Safety Constraints

- Local apply must always verify manifests and checksums.
- Local apply must back up files before writes.
- Local apply must be rollback-capable.
- Direct remote config must be explicitly enabled per template.
- Templates may not manage runtime state unless a future spec explicitly allows a narrow exception.

## Product Constraints

- Users should be able to open the hub URL and discover what to do next.
- CLI workflows should work without the web UI.
- Web UI workflows should provide copyable CLI commands.
- The hub should be useful on a small VPS or LAN machine.

## When To Revisit

Reconsider these constraints only when:

- file-backed storage cannot handle real usage;
- server-rendered UI blocks required workflows;
- optional external adapters become necessary for a documented user need;
- multi-user authorization requires a stronger model than token-based MVP access.
