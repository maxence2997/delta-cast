# Agora NCS — RTC Channel Event Notifications Quick Reference

> **Sources**:
>
> - <https://docs.agora.io/en/interactive-live-streaming/advanced-features/receive-notifications>
> - <https://docs.agora.io/en/interactive-live-streaming/channel-management-api/webhook/channel-event-type>

---

## Overview

Agora NCS sends `POST` HTTPS callbacks to your webhook for subscribed RTC channel events.
Your server must return `200 OK` with a JSON body within **10 seconds**. Retry policy: up to 3 retries with increasing intervals.

**DeltaCast endpoint**: `POST /v1/webhook/agora/channel`

---

## Request Headers

| Header               | Description                                                      |
| -------------------- | ---------------------------------------------------------------- |
| `Content-Type`       | `application/json`                                               |
| `Agora-Signature`    | HMAC/SHA1 of raw request body using `AGORA_CHANNEL_NCS_SECRET`   |
| `Agora-Signature-V2` | HMAC/SHA256 of raw request body using `AGORA_CHANNEL_NCS_SECRET` |

## Request Body Fields

| Field       | Type        | Description                                                        |
| ----------- | ----------- | ------------------------------------------------------------------ |
| `noticeId`  | String      | Unique notification ID. Use with `notifyMs` to deduplicate.        |
| `productId` | Number      | Always `1` for RTC Channel events.                                 |
| `eventType` | Number      | Type of event — see table below.                                   |
| `notifyMs`  | Number      | Unix timestamp (ms) when NCS sent the callback. Updated on resend. |
| `sid`       | String      | Session ID.                                                        |
| `payload`   | JSON Object | Event-specific payload — see event details below.                  |

---

## Event Types

| Code  | Name                       | Description                              |
| ----- | -------------------------- | ---------------------------------------- |
| `101` | channel create             | First user joins; channel initialized    |
| `102` | channel destroy            | Last user leaves; channel destroyed      |
| `103` | broadcaster join channel   | Host joins in LIVE_BROADCASTING profile  |
| `104` | broadcaster leave channel  | Host leaves in LIVE_BROADCASTING profile |
| `105` | audience join channel      | Audience member joins                    |
| `106` | audience leave channel     | Audience member leaves                   |
| `107` | user join (communication)  | User joins in COMMUNICATION profile      |
| `108` | user leave (communication) | User leaves in COMMUNICATION profile     |
| `111` | role → broadcaster         | Audience switches to host role           |
| `112` | role → audience            | Host switches to audience role           |

---

## Event 103 — Broadcaster Join Channel

```json
{
  "channelName": "deltacast-abc123",
  "uid": 12121212,
  "platform": 7,
  "clientSeq": 1625051030746,
  "ts": 1560396843,
  "account": ""
}
```

| Field         | Type            | Description                                                        |
| ------------- | --------------- | ------------------------------------------------------------------ |
| `channelName` | String          | Channel name                                                       |
| `uid`         | Number          | Host UID                                                           |
| `platform`    | Number          | `1`=Android `2`=iOS `5`=Windows `6`=Linux `7`=Web `8`=macOS        |
| `clientSeq`   | Number (UInt64) | Monotonically increasing per user; use to order/deduplicate events |
| `ts`          | Number          | Unix timestamp (s) of the event                                    |

---

## Event 104 — Broadcaster Leave Channel

```json
{
  "channelName": "deltacast-abc123",
  "uid": 12121212,
  "platform": 7,
  "clientSeq": 1625051030789,
  "reason": 1,
  "ts": 1560396943,
  "duration": 600,
  "account": ""
}
```

Same fields as event 103, plus:

| Field      | Type   | Description                                             |
| ---------- | ------ | ------------------------------------------------------- |
| `reason`   | Number | `1`=quit normally `2`=timeout `3`=kicked `5`=new device |
| `duration` | Number | Time spent in channel (seconds)                         |

---

## Signature Verification

Verify `Agora-Signature` (HMAC/SHA1) using `AGORA_CHANNEL_NCS_SECRET`. Use constant-time comparison to avoid timing attacks.

---

## DeltaCast Integration Notes

- **Event 103**: Session `ready` → triggers Media Push to GCP + YouTube; state → `live`.
- **Event 102**: Session `live` → triggers auto-stop (full teardown).
- **Event 104**: Session `live` AND `clientSeq > 0` → triggers auto-stop. `clientSeq == 0` = Media Push bot leave event; ignored.
- Other events are ignored (return `200`).
- Idempotency guard: Session state is checked before acting — duplicate events are safe.
- Health check events (`channelName: "test_webhook"`) are filtered out before processing.
- Files: `server/internal/handler/webhook_handler.go`, `server/internal/service/live_service.go`
