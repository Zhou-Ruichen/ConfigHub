# Admin UI Guidelines

## Goal

The UI exists so a user can open the hub and immediately understand how to consume config.

## Required Information

Show:

- hub status;
- available profiles;
- available template domains;
- latest bundle version per profile;
- copyable bootstrap command;
- direct-read URLs where enabled;
- safety warnings for templates that write local files.

## Interaction Rules

- Prefer server-rendered HTML.
- Keep forms simple and explicit.
- Copy buttons may use minimal embedded JavaScript.
- Do not hide target paths or safety policy behind decoration.
- Do not require login/session complexity before token-based API access exists.

## Visual Direction

Use a utilitarian admin-tool layout:

- compact tables;
- clear status badges;
- monospace for commands, paths, and checksums;
- restrained color usage;
- no marketing hero as the main product screen.
