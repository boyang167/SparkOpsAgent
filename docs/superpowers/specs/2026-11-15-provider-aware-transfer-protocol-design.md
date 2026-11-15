# Provider-Aware Transfer Protocol Design

## Problem

Uploads hard-code `DIRECT` + port 8082 while Placement may select non-default `LOCAL` roots or `S3` providers. The control plane must pick a protocol and data plane per provider.

## Current Building Blocks

- **8082 gateway**: `ResolveUpload` loads the session provider from catalog and calls `Provider.Put` (supports any `LOCAL` endpoint).
- **Placement**: `RequiresPresignedUpload` when protocol is `MULTIPART`; filters providers with `presigned_upload`.
- **web-sdk**: `uploadTransfer` supports `DIRECT` and `MULTIPART` (per-part `upload_url`).
- **Gap**: HTTP create response does not issue multipart presign URLs; S3 browser upload path is incomplete.

## Target Behavior

| Placed provider | Protocol | Client data plane |
|-----------------|----------|-------------------|
| Any active `LOCAL` | `DIRECT` | `PUT` 8082 with capability token; gateway writes to provider endpoint |
| Default `LOCAL` (= gateway root) | `DIRECT` | Same (optimized path) |
| `S3` / remote with presign | `MULTIPART` | Browser `PUT` each presigned part URL; control plane completes multipart |
| `S3` without presign | Reject or org policy | No browser DIRECT to gateway for large objects |

## Phase A (done in code)

- `CreateTransfer`: allow `DIRECT` for any org-active `LOCAL` provider (gateway proxies).
- Auto-upgrade `DIRECT` → `MULTIPART` when Placement selects non-`LOCAL` provider; re-run Placement with `RequiresPresignedUpload`.

## Phase B (multipart issuance)

- After session create for `MULTIPART`: call storage `CreateMultipartUpload`, persist parts, return `upload_url` per part in transfer view (S3 presign; optional local part tokens via control API).
- Commands: `CompleteTransferMultipart`, integrate with `RecordTransferUploaded`.
- OpenAPI + contracts: `parts[].upload_url`, optional `multipart_upload_id`.

## Phase C (frontend)

- Stop hard-coding `protocol: "DIRECT"`; use session `protocol` from create response.
- Workbench upload: `uploadTransfer` branch already exists; surface clearer errors when part URLs missing.
- Transfer center: show provider / protocol in metadata.

## Success Criteria

- Binding to alternate `LOCAL` pool uploads succeed via 8082 without gateway mismatch.
- S3-bound workspace: create returns `MULTIPART` with presigned parts; file completes end-to-end.
- Regression: default-local binding unchanged.