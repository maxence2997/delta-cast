# Agora Media Push — Event Notifications Quick Reference

> **Source**: <https://docs.agora.io/en/media-push/develop/receive-notifications>

---

## Overview

Agora NCS sends `POST` HTTPS callbacks to your webhook for Media Push Converter lifecycle events.
Your server must return `200 OK` with a JSON body within **10 seconds**. Retry policy: up to 3 retries with increasing intervals.

**DeltaCast endpoint**: `POST /v1/webhook/agora/media-push`

---

## Request Headers

| Header               | Description                                                         |
| -------------------- | ------------------------------------------------------------------- |
| `Content-Type`       | `application/json`                                                  |
| `Agora-Signature`    | HMAC/SHA1 of raw request body using `AGORA_MEDIA_PUSH_NCS_SECRET`   |
| `Agora-Signature-V2` | HMAC/SHA256 of raw request body using `AGORA_MEDIA_PUSH_NCS_SECRET` |

> This secret is **independent** from `AGORA_CHANNEL_NCS_SECRET`.

## Request Body Fields

| Field       | Type        | Description                                                             |
| ----------- | ----------- | ----------------------------------------------------------------------- |
| `noticeId`  | String      | Notification ID. Use with `notifyMs` to deduplicate.                    |
| `productId` | Number      | Always `5` for Media Push events.                                       |
| `eventType` | Number      | Type of Converter event — see table below.                              |
| `notifyMs`  | Number      | Unix timestamp (ms) when Agora sent the notification. Updated on retry. |
| `payload`   | JSON Object | Event-specific content — see event details below.                       |

---

## Event Types

| Code | Name                            | Description                                                 |
| ---- | ------------------------------- | ----------------------------------------------------------- |
| `1`  | Converter created               | Converter created via the `Create` API call                 |
| `2`  | Converter configuration changed | Configuration updated via the `Update` API call             |
| `3`  | Converter status changed        | Running state changed (`connecting` → `running` / `failed`) |
| `4`  | Converter destroyed             | Converter destroyed; RTMP push stopped                      |

---

## Event 3 — Converter Status Changed

The most operationally important event — signals `running` (push is live) or `failed` (push error).

```json
{
  "converter": {
    "id": "4c014467d647bb87b60b719f6fa57686",
    "createTs": 1603456600,
    "updateTs": 1603456600,
    "state": "running"
  },
  "lts": 1603456600,
  "fields": "id,createTs,updateTs,state"
}
```

| `converter.state` | Description                     |
| ----------------- | ------------------------------- |
| `"connecting"`    | Connecting to the Agora server  |
| `"running"`       | Streams are being pushed to CDN |
| `"failed"`        | Stream push to CDN has failed   |

---

## Event 4 — Converter Destroyed

```json
{
  "converter": {
    "id": "4c014467d647bb87b60b719f6fa57686",
    "name": "deltacast-abc123-gcp",
    "createTs": 1603456600,
    "updateTs": 1603456600
  },
  "lts": 1603456600,
  "destroyReason": "Delete Request",
  "fields": "id,name,createTs,updateTs"
}
```

| `destroyReason`    | Description                                                   |
| ------------------ | ------------------------------------------------------------- |
| `"Delete Request"` | Destroyed by a `Delete` API call (`StopMediaPush` was called) |
| `"Idle Timeout"`   | All users left the channel and `idleTimeOut` elapsed          |
| `"Internal Error"` | Agora server error — requires investigation or retry          |

---

## Signature Verification

Verify `Agora-Signature` (HMAC/SHA1) using `AGORA_MEDIA_PUSH_NCS_SECRET`. Use constant-time comparison. Callbacks may arrive out of order and may be duplicated — use `noticeId` + `notifyMs` to deduplicate.

---

## DeltaCast Integration Notes

- Currently all events are **logged only** — no state transitions are triggered from this webhook.
- `state == "failed"` on event 3 is logged as an error; future work may trigger automatic Converter recreation.
- File: `server/internal/handler/webhook_handler.go`
