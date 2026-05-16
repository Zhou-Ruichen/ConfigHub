# Apply Engine Guidelines

The apply engine is the highest-risk part of ConfigHub.

## Required Flow

1. Load bundle.
2. Validate manifest schema.
3. Verify checksums.
4. Validate target path policy.
5. Produce diff.
6. Require confirmation or `--yes`.
7. Back up current target files.
8. Write temp files in target directories.
9. Atomically rename into place.
10. Write apply log.
11. Expose rollback pointer.

## Tests

Use `t.TempDir()` for every write-path test.

Required test classes:

- valid apply;
- checksum mismatch rejection;
- forbidden target rejection;
- symlink rejection by default;
- rollback success;
- partial failure does not lose backups.

## Logging

Apply logs record file paths, bundle versions, timestamps, and checksums. They must not record raw secret values.
