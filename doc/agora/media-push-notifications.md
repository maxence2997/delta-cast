# Agora Media Push — Event Notifications Reference

> Source: <https://docs.agora.io/en/media-push/develop/receive-notifications>

---

## Overview

Agora Notifications can send your webhook server callbacks for Media Push lifecycle events. When a
Converter is created, its configuration is updated, its running state changes, or it is destroyed,
Agora sends an HTTPS POST request to your registered endpoint.

This webhook is configured under **Agora Console → Notifications → Media Push Restful API**.
The secret shown there is `AGORA_MEDIA_PUSH_NCS_SECRET` in DeltaCast.

> **Endpoint:** `POST /v1/webhook/agora/media-push`

---

## Workflow

1. A user action (or Agora server event) creates a Media Push lifecycle event.
2. Agora sends an HTTPS POST request to your webhook endpoint.
3. Your server verifies the signature from the request header.
4. Your server processes the payload and returns `200 OK` within **10 seconds**. The response body must be JSON.
5. If Agora does not receive `200 OK` in time, it retries up to 3 times with increasing intervals.

---

## Request Headers

| Header               | Description                                                                               |
| -------------------- | ----------------------------------------------------------------------------------------- |
| `Content-Type`       | `application/json`                                                                        |
| `Agora-Signature`    | HMAC/SHA1 signature computed from the raw request body using the Media Push NCS secret.   |
| `Agora-Signature-V2` | HMAC/SHA256 signature computed from the raw request body using the Media Push NCS secret. |

> DeltaCast verifies `Agora-Signature` (HMAC/SHA1). The Media Push NCS secret is **separate** from
> the RTC Channel NCS secret.

---

## Request Body Fields

| Field       | Type        | Description                                                                                    |
| ----------- | ----------- | ---------------------------------------------------------------------------------------------- |
| `noticeId`  | String      | Notification ID. Use with `notifyMs` to deduplicate repeated callbacks.                        |
| `productId` | Number      | Always `5` for Media Push events.                                                              |
| `eventType` | Number      | The type of Converter event. See [Event Types](#event-types) below.                            |
| `notifyMs`  | Number      | Unix timestamp (ms) when the notification was sent by Agora. Updated on retries.               |
| `payload`   | JSON Object | Event-specific content. Structure varies by event type. See each event type for field details. |

### Request Body Example

```json
{
  "noticeId": "2000001428:4330:107",
  "productId": 5,
  "eventType": 3,
  "notifyMs": 1611566412672,
  "payload": { "..." }
}
```

---

## Event Types

| Code | Name                            | Description                                                                     |
| ---- | ------------------------------- | ------------------------------------------------------------------------------- |
| 1    | Converter created               | A Converter was created via the `Create` API call.                              |
| 2    | Converter configuration changed | The Converter's configuration was updated via the `Update` API call.            |
| 3    | Converter status changed        | The running state of a Converter changed (`connecting` → `running` / `failed`). |
| 4    | Converter destroyed             | A Converter was destroyed and the RTMP push stopped.                            |

---

## Event Payloads

### Event 1 — Converter Created

The `eventType` is `1`. The payload contains the full Converter configuration at creation time.

Key `payload.converter` fields:

| Field                        | Type   | Description                                                      |
| ---------------------------- | ------ | ---------------------------------------------------------------- |
| `converter.id`               | String | UUID of the Converter assigned by Agora.                         |
| `converter.name`             | String | The name given to the Converter on creation.                     |
| `converter.rtmpUrl`          | String | The RTMP push destination URL.                                   |
| `converter.idleTimeout`      | Number | Idle timeout (s) before the Converter is auto-destroyed.         |
| `converter.createTs`         | Number | Unix timestamp (s) when the Converter was created.               |
| `converter.updateTs`         | Number | Unix timestamp (s) of the last configuration update.             |
| `converter.state`            | String | State at creation time — always `"connecting"`.                  |
| `converter.transcodeOptions` | Object | Transcoding config (present for transcoded Converters).          |
| `converter.rawOptions`       | Object | Raw (non-transcoded) stream config (present for raw Converters). |
| `lts`                        | Number | Unix timestamp (ms) when the event occurred on the Agora server. |
| `xRequestId`                 | String | UUID echoed from the `Create` request's `X-Request-ID` header.   |

```json
{
  "converter": {
    "id": "4c014467d647bb87b60b719f6fa57686",
    "name": "deltacast-abc123",
    "rtmpUrl": "rtmp://example.agora.io/live/show68",
    "idleTimeout": 300,
    "createTs": 1591786766,
    "updateTs": 1591786835,
    "state": "connecting"
  },
  "lts": 1603456600,
  "xRequestId": "7bbcc8a4acce48c78b53c5a261a8a564"
}
```

---

### Event 2 — Converter Configuration Changed

The `eventType` is `2`. Only the changed fields are included in `payload.converter`. The `fields`
string (FieldMask) lists which sub-fields were updated.

Key `payload` fields:

| Field                | Type   | Description                                                            |
| -------------------- | ------ | ---------------------------------------------------------------------- |
| `converter.id`       | String | UUID of the Converter.                                                 |
| `converter.state`    | String | Current running state (`running` while active).                        |
| `converter.createTs` | Number | Unix timestamp (s) when the Converter was created.                     |
| `converter.updateTs` | Number | Unix timestamp (s) of this update.                                     |
| `fields`             | String | FieldMask listing the updated sub-fields (e.g., `"id,state,rtmpUrl"`). |
| `lts`                | Number | Unix timestamp (ms) when the event occurred on the Agora server.       |
| `xRequestId`         | String | UUID from the `Update` request's `X-Request-ID` header.                |

```json
{
  "converter": {
    "id": "4c014467d647bb87b60b719f6fa57686",
    "createTs": 1591786766,
    "updateTs": 1591786835,
    "state": "running",
    "rtmpUrl": "rtmp://example.agora.io/live/show68_new"
  },
  "lts": 1603456600,
  "xRequestId": "7bbcc8a4acce48c78b53c5a261a8a564",
  "fields": "id,createTs,updateTs,state,rtmpUrl"
}
```

---

### Event 3 — Converter Status Changed

The `eventType` is `3`. This is the most operationally important event — it signals when a
Converter transitions to `running` (push is live) or `failed` (push error).

Key `payload` fields:

| Field                | Type   | Description                                                       |
| -------------------- | ------ | ----------------------------------------------------------------- |
| `converter.id`       | String | UUID of the Converter.                                            |
| `converter.state`    | String | New state. See state values below.                                |
| `converter.createTs` | Number | Unix timestamp (s) when the Converter was created.                |
| `converter.updateTs` | Number | Unix timestamp (s) when the state changed.                        |
| `fields`             | String | FieldMask — always `"id,createTs,updateTs,state"` for this event. |
| `lts`                | Number | Unix timestamp (ms) when the event occurred on the Agora server.  |

**Converter state values:**

| State          | Description                                            |
| -------------- | ------------------------------------------------------ |
| `"connecting"` | Converter is connecting to the Agora server.           |
| `"running"`    | Converter is running; streams are being pushed to CDN. |
| `"failed"`     | Stream push to CDN has failed.                         |

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

> **DeltaCast usage:** When `state == "failed"`, the service logs an error. Future work may trigger
> automatic recovery (recreating the Converter).

---

### Event 4 — Converter Destroyed

The `eventType` is `4`. Sent when a Converter is destroyed for any reason.

Key `payload` fields:

| Field                | Type   | Description                                                      |
| -------------------- | ------ | ---------------------------------------------------------------- |
| `converter.id`       | String | UUID of the Converter.                                           |
| `converter.name`     | String | The name given to the Converter.                                 |
| `converter.createTs` | Number | Unix timestamp (s) when the Converter was created.               |
| `converter.updateTs` | Number | Unix timestamp (s) of the last configuration update.             |
| `destroyReason`      | String | The reason for destruction. See destroy reason values below.     |
| `fields`             | String | FieldMask — `"id,name,createTs,updateTs"` for this event.        |
| `lts`                | Number | Unix timestamp (ms) when the event occurred on the Agora server. |

**Destroy reason values:**

| Value              | Description                                                                          |
| ------------------ | ------------------------------------------------------------------------------------ |
| `"Delete Request"` | The Agora server received a `Delete` API request (i.e., `StopMediaPush` was called). |
| `"Idle Timeout"`   | All users left the channel and the `idleTimeOut` period elapsed.                     |
| `"Internal Error"` | Agora server error (e.g., hardware failure). Requires investigation or retry.        |

```json
{
  "converter": {
    "id": "4c014467d647bb87b60b719f6fa57686",
    "name": "deltacast-abc123",
    "createTs": 1603456600,
    "updateTs": 1603456600
  },
  "lts": 1603456600,
  "destroyReason": "Delete Request",
  "fields": "id,name,createTs,updateTs"
}
```

---

## Signature Verification

Agora signs the raw POST body using HMAC/SHA1 with the **Media Push NCS secret** (`AGORA_MEDIA_PUSH_NCS_SECRET`):

```go
mac := hmac.New(sha1.New, []byte(secret))
mac.Write(body)
signature := hex.EncodeToString(mac.Sum(nil))
// compare with Agora-Signature header using constant-time comparison
```

> This secret is **independent** from the RTC Channel NCS secret (`AGORA_CHANNEL_NCS_SECRET`).
> Each Agora notification product has its own secret.

---

## Reliability Considerations

- **Out-of-order delivery**: Callbacks are not guaranteed to arrive in event order.
- **Duplicate delivery**: Agora may deliver the same notification more than once. Use `noticeId` + `notifyMs` together to deduplicate.
- **10-second timeout**: Your server must respond `200 OK` with a JSON body within 10 seconds.
- **Retries**: Up to 3 retries with gradually increasing intervals.
