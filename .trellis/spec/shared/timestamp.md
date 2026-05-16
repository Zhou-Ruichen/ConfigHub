# Timestamp Specification

## Rule

Use RFC3339 UTC strings for persisted metadata and API payloads unless a lower-level file format has a stronger reason to use Unix milliseconds.

Examples:

```json
{
  "createdAt": "2026-05-16T14:30:00Z",
  "appliedAt": "2026-05-16T14:31:12Z"
}
```

## Why RFC3339

ConfigHub metadata is read by humans in manifests, logs, and web pages. RFC3339 is easier to inspect than raw milliseconds and remains language-neutral.

## Go Rules

- Use `time.Time` internally.
- Normalize persisted timestamps to UTC.
- Use `time.RFC3339` or `time.RFC3339Nano` consistently per file type.
- Do not store local timezone-dependent strings in manifests.

## Where Timestamps Appear

- bundle manifest `createdAt`;
- bundle apply logs;
- backup directory metadata;
- audit log entries;
- server status responses.

## Exceptions

Unix milliseconds may be used for:

- performance-sensitive internal measurements;
- duration calculations;
- compatibility with an external format that requires milliseconds.

Do not mix timestamp units inside the same persisted contract.
