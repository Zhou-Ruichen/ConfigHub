# Bundle Contract

## Concepts

- **Profile**: who or what the bundle is for, such as a user, device, role, OS, or team.
- **Template**: source definition for one rendered config output.
- **Bundle**: immutable rendered output for one profile/version.
- **Manifest**: machine-readable contract describing every file in the bundle.
- **Delivery mode**: `sync`, `remote`, or both.

## Manifest Requirements

Every bundle manifest must include:

- `schemaVersion`
- `bundleVersion`
- `profileId`
- `createdAt` (RFC3339 UTC; see [timestamp.md](./timestamp.md))
- `sourceRevision`
- `domains` (array of domain ids included in this bundle)
- `files` (array of file entries; see below)
- `removedFiles` (array of entries describing files that were present in a prior bundle but should now be deleted on apply)
- `changeSummary` (human-readable summary, may be empty)
- `signature` (optional in Slice 2-4; required from Slice 5 onward)
- `redactedFiles` (only present in API responses where the requester lacks access to some entries)

Every file entry under `files` must include:

- `templateId`
- `domain`
- `bundlePath` (relative path inside the bundle)
- `targetPath` (absolute target path, may contain `~` which the apply engine expands per the active user)
- `mode` (octal string, e.g. `"0644"`)
- `checksum` (`sha256:<hex>`)
- `delivery` (`sync` | `remote` | `both`)
- `safety` block (`backup`, `diff`, `symlink`, `secrets`, `merge`, `includeStrategy`)

Every entry under `removedFiles` must include:

- `templateId` (the previous id; may be absent if the template was deleted entirely)
- `targetPath` (the path to delete on apply)
- `reason` (`renamed` | `removed`)
- `previousChecksum` (`sha256:<hex>`; clients refuse to delete a file whose current checksum does not match)

## Checksum Rules

- Hash every rendered file with sha256.
- Verify hashes before local apply.
- Verify hashes after hub pull.
- Do not apply files missing from the manifest.
- Do not apply manifest entries missing rendered files.
- Refuse to delete `removedFiles` entries when the local file's checksum does not match `previousChecksum`; surface the mismatch to the operator.

## Versioning

- Bundle versions should be monotonic within a profile.
- Schema version changes must be explicit and bump `schemaVersion`.
- Clients must reject unsupported schema versions.

## Direct Remote Reads

Remote template reads are allowed only when the template declares `delivery.remote`.

Remote responses must:

- identify the profile and template id;
- avoid leaking secret-derived values unless the template explicitly permits them and the requester is authorized (see [../big-question/secret-handling.md](../big-question/secret-handling.md));
- include an `ETag` header (sha256 of the rendered bytes) so clients can cache safely.

## Worked Example

```json
{
  "schemaVersion": "1",
  "bundleVersion": "2026-05-16T15-04-22Z-001",
  "profileId": "macbook",
  "createdAt": "2026-05-16T15:04:22Z",
  "sourceRevision": "git:abc1234",
  "domains": ["ai", "dotfiles"],
  "files": [
    {
      "templateId": "ai/claude",
      "domain": "ai",
      "bundlePath": "files/ai/claude/settings.json",
      "targetPath": "~/.claude/settings.json",
      "mode": "0600",
      "checksum": "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
      "delivery": "sync",
      "safety": {
        "backup": "required",
        "diff": "required",
        "symlink": "reject",
        "secrets": "allowed",
        "merge": "replace"
      }
    },
    {
      "templateId": "dotfiles/git",
      "domain": "dotfiles",
      "bundlePath": "files/dotfiles/git/confighub.gitconfig",
      "targetPath": "~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig",
      "mode": "0644",
      "checksum": "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
      "delivery": "sync",
      "safety": {
        "backup": "required",
        "diff": "required",
        "symlink": "reject",
        "secrets": "forbidden",
        "merge": "replace"
      }
    },
    {
      "templateId": "dotfiles/git-include",
      "domain": "dotfiles",
      "bundlePath": "files/dotfiles/git/include-line.txt",
      "targetPath": "~/.gitconfig",
      "mode": "0644",
      "checksum": "sha256:fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9",
      "delivery": "sync",
      "safety": {
        "backup": "required",
        "diff": "required",
        "symlink": "reject",
        "secrets": "forbidden",
        "merge": "managed-section",
        "includeStrategy": "append-once"
      }
    }
  ],
  "removedFiles": [
    {
      "templateId": "ai/gemini-legacy",
      "targetPath": "~/.config/gemini/old.json",
      "reason": "removed",
      "previousChecksum": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    }
  ],
  "changeSummary": "Update Claude settings; remove legacy Gemini config.",
  "signature": null
}
```

The example demonstrates: a secret-bearing replace-mode JSON file, a plain-replace fragment file, an include-line entry using `managed-section` + `append-once`, and one removed file. JSON targets cannot use `managed-section` in Slice 3 (marker comments are not supported in JSON); they must use `replace` until `deep-merge` lands.
