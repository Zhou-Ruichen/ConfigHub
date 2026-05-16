# Config Distribution Guide

## Core Question

Is this configuration consumed by syncing local files, direct remote read, or both?

## Sync Mode

Use sync mode when:

- the target tool only reads local files;
- local offline use matters;
- the file must be present before the tool starts.

Sync mode requires:

- manifest entry;
- checksum;
- diff;
- backup;
- atomic write;
- rollback.

## Remote Mode

Use remote mode when:

- the tool or wrapper can read a URL;
- the config is safe to serve to the authorized requester;
- latency and hub availability are acceptable.

Remote mode requires:

- explicit template opt-in;
- authorization checks;
- redaction policy;
- checksum/ETag when practical.

## Both Modes

Allow both when remote read is convenient but local fallback is still needed.

Do not assume remote mode removes local apply safety requirements.
