# Agora NCS — RTC Channel Event Notifications

Sources:

- https://docs.agora.io/en/interactive-live-streaming/advanced-features/receive-notifications
- https://docs.agora.io/en/interactive-live-streaming/channel-management-api/webhook/channel-event-type

---

## Overview

Agora Notification Center Service (NCS) sends HTTPS POST callbacks to your webhook when subscribed RTC channel events occur. Your server must authenticate the notification and return `200 OK` within **10 seconds**. The response body must be in **JSON format**.

Retry policy: If `200 OK` is not received within 10 s, Agora immediately resends. The interval between retries increases gradually. Notifications stops after **3 retries**.

This webhook is configured under **Agora Console → Notifications → RTC Channel Event Callbacks**.
The secret shown there corresponds to `AGORA_CHANNEL_NCS_SECRET` in DeltaCast.

---

## Request Format

### HTTP Method

`POST` (HTTPS only — HTTP is not supported)

### Headers

| Header               | Description                                                            |
| -------------------- | ---------------------------------------------------------------------- |
| `Content-Type`       | `application/json`                                                     |
| `Agora-Signature`    | HMAC/SHA1 signature computed from the NCS secret + raw request body.   |
| `Agora-Signature-V2` | HMAC/SHA256 signature computed from the NCS secret + raw request body. |

### Body Fields

| Field       | Type        | Description                                                                   |
| ----------- | ----------- | ----------------------------------------------------------------------------- |
| `noticeId`  | String      | Unique ID for this notification callback. Use with `notifyMs` to deduplicate. |
| `productId` | Number      | Always `1` for RTC Channel events.                                            |
| `eventType` | Number      | Type of event (see Event Types below).                                        |
| `notifyMs`  | Number      | Unix timestamp (ms) when NCS sends the callback. Updated on resend.           |
| `sid`       | String      | Session ID.                                                                   |
| `payload`   | JSON Object | Event-specific payload (varies per event type).                               |

#### Example

```json
{
  "noticeId": "2000001428:4330:107",
  "productId": 1,
  "eventType": 101,
  "notifyMs": 1611566412672,
  "payload": { "...": "..." }
}
```

---

## Signature Verification

1. Agora generates two signatures from the **NCS secret** (obtained from Agora Console):
   - `Agora-Signature`: HMAC/SHA1 of the raw request body
   - `Agora-Signature-V2`: HMAC/SHA256 of the raw request body
2. Your server reads **the raw body first**, then verifies the signature.
3. You only need to verify **one** of the two headers (V1 or V2).

### Go example (SHA1)

```go
mac := hmac.New(sha1.New, []byte(secret))
mac.Write(body)
expected := hex.EncodeToString(mac.Sum(nil))
// Use hmac.Equal for constant-time comparison (timing-attack safe).
return hmac.Equal([]byte(expected), []byte(receivedSignature))
```

---

## Response Requirements

- HTTP status: **`200 OK`**
- Body: **JSON format** (e.g., `{"status": "ok"}`)
- Must respond within **10 seconds**
- Best practice: enable HTTP keep-alive (`MaxKeepAliveRequests ≥ 100`, `KeepAliveTimeout ≥ 10s`)

---

## Event Types

| Code  | Name                       | Description                                          |
| ----- | -------------------------- | ---------------------------------------------------- |
| `101` | channel create             | First user joins; channel initialized.               |
| `102` | channel destroy            | Last user leaves; channel destroyed.                 |
| `103` | broadcaster join channel   | Host joins in streaming (LIVE_BROADCASTING) profile. |
| `104` | broadcaster leave channel  | Host leaves in streaming profile.                    |
| `105` | audience join channel      | Audience member joins in streaming profile.          |
| `106` | audience leave channel     | Audience member leaves in streaming profile.         |
| `107` | user join (communication)  | User joins in communication profile.                 |
| `108` | user leave (communication) | User leaves in communication profile.                |
| `111` | role → broadcaster         | Audience switches to host role.                      |
| `112` | role → audience            | Host switches to audience role.                      |

### Recommended subscriptions

- **LIVE_BROADCASTING profile**: `103`, `104`, `105`, `106`, `111`, `112`
- **COMMUNICATION profile**: `103`, `104`, `111`, `112`

---

## Event Payloads

### 101 — channel create

| Field         | Type   | Description                                               |
| ------------- | ------ | --------------------------------------------------------- |
| `channelName` | String | The channel name.                                         |
| `ts`          | Number | Unix timestamp (s) when the event occurred on the server. |

```json
{ "channelName": "test_webhook", "ts": 1560396834 }
```

---

### 102 — channel destroy

| Field         | Type   | Description                                                                                 |
| ------------- | ------ | ------------------------------------------------------------------------------------------- |
| `channelName` | String | The channel name.                                                                           |
| `ts`          | Number | Unix timestamp (s) when the event occurred.                                                 |
| `lastUid`     | Number | UID of the last user to leave. Multiple callbacks may appear if users leave simultaneously. |

```json
{ "channelName": "test_webhook", "ts": 1560399999, "lastUid": 12121212 }
```

---

### 103 — broadcaster join channel

| Field         | Type            | Description                                                                            |
| ------------- | --------------- | -------------------------------------------------------------------------------------- |
| `channelName` | String          | The channel name.                                                                      |
| `uid`         | Number          | The host's UID.                                                                        |
| `platform`    | Number          | `1`=Android, `2`=iOS, `5`=Windows, `6`=Linux, `7`=Web, `8`=macOS, `0`=Other            |
| `clientType`  | Number          | Linux only (`platform=6`): `3`=On-premise Recording, `8`=Applets, `10`=Cloud Recording |
| `clientSeq`   | Number (UInt64) | Sequence number; monotonically increasing per user. Use to order/deduplicate events.   |
| `ts`          | Number          | Unix timestamp (s) when the event occurred on the server.                              |
| `account`     | String          | The user account / user ID string.                                                     |

```json
{
  "channelName": "test_webhook",
  "uid": 12121212,
  "platform": 1,
  "clientSeq": 1625051030746,
  "ts": 1560396843,
  "account": "test"
}
```

> **DeltaCast usage:** This event triggers Media Push to GCP and YouTube when the streamer joins.

---

### 104 — broadcaster leave channel

| Field         | Type            | Description                                             |
| ------------- | --------------- | ------------------------------------------------------- |
| `channelName` | String          | The channel name.                                       |
| `uid`         | Number          | The host's UID.                                         |
| `platform`    | Number          | Same codes as event 103.                                |
| `clientType`  | Number          | Linux only (`platform=6`).                              |
| `clientSeq`   | Number (UInt64) | Sequence number for ordering events from the same user. |
| `reason`      | Number          | Leave reason — see Reason Codes below.                  |
| `ts`          | Number          | Unix timestamp (s) when the event occurred.             |
| `duration`    | Number          | Time the user spent in the channel (seconds).           |
| `account`     | String          | The user account / user ID string.                      |

```json
{
  "channelName": "test_webhook",
  "uid": 12121212,
  "platform": 1,
  "clientSeq": 1625051030789,
  "reason": 1,
  "ts": 1560396943,
  "duration": 600,
  "account": "test"
}
```

---

### 105 — audience join channel

Same fields as [103 — broadcaster join channel](#103--broadcaster-join-channel), with `uid` being the audience member's UID.

```json
{
  "channelName": "test_webhook",
  "uid": 12121212,
  "platform": 1,
  "clientSeq": 1625051035346,
  "ts": 1560396843,
  "account": "test"
}
```

---

### 106 — audience leave channel

Same fields as [104 — broadcaster leave channel](#104--broadcaster-leave-channel), with `uid` being the audience member's UID.

```json
{
  "channelName": "test_webhook",
  "uid": 12121212,
  "platform": 1,
  "clientSeq": 1625051035390,
  "reason": 1,
  "ts": 1560396943,
  "duration": 600,
  "account": "test"
}
```

---

### 111 — client role change to broadcaster

| Field         | Type            | Description                                             |
| ------------- | --------------- | ------------------------------------------------------- |
| `channelName` | String          | The channel name.                                       |
| `uid`         | Number          | The user's UID.                                         |
| `clientSeq`   | Number (UInt64) | Sequence number for ordering events from the same user. |
| `ts`          | Number          | Unix timestamp (s) when the event occurred.             |
| `account`     | String          | The user account / user ID string.                      |

```json
{
  "channelName": "test_webhook",
  "uid": 12121212,
  "clientSeq": 1625051035469,
  "ts": 1560396834,
  "account": "test"
}
```

---

### 112 — client role change to audience

Same fields as [111 — client role change to broadcaster](#111--client-role-change-to-broadcaster).

```json
{
  "channelName": "test_webhook",
  "uid": 12121212,
  "clientSeq": 16250510358369,
  "ts": 1560496834,
  "account": "test"
}
```

---

## Reason Codes (leave events: 104, 106, 108)

| Code  | Meaning                                                                  |
| ----- | ------------------------------------------------------------------------ |
| `1`   | User quit normally.                                                      |
| `2`   | Connection timeout (no data for >10 s).                                  |
| `3`   | Permission — kicked via Banning API.                                     |
| `4`   | Agora RTC server internal issue (transient).                             |
| `5`   | User joined from a new device; old device forced out.                    |
| `9`   | SDK disconnected due to multiple client IPs.                             |
| `999` | **Abnormal** — frequent login/logout. Treat as special case (see below). |

---

## Handling Redundant / Out-of-Order Notifications

NCS **may send duplicates** and **does not guarantee delivery order**.

### Strategy 1 — `clientSeq` ordering (per-user)

1. Track per-user state: `{uid, role, isOnline, lastClientSeq}` keyed by `(channelName, uid)`.
2. On each callback:
   - `clientSeq` > `lastClientSeq` → process it.
   - `clientSeq` ≤ `lastClientSeq` → discard (duplicate or out-of-order).
3. On user-leave events: wait **1 minute** before deleting user data (to handle late duplicates).

### Strategy 2 — `noticeId` + `notifyMs` deduplication

Use a combination of `noticeId` and `notifyMs` to deduplicate: same `noticeId` with same or older `notifyMs` is a duplicate.

### Abnormal user activity (`reason=999`)

When event `104` arrives with `reason=999`:

1. Do **not** immediately remove the user.
2. Wait **1 minute**, then call the Banning user privileges API to remove the user from the channel.
3. If deleted immediately, subsequent out-of-order callbacks may corrupt online-status tracking.

---

## Health Check (Console)

When enabling NCS in Agora Console, a health-test event is sent before saving configuration:

- `channelName`: `test_webhook`
- `uid`: `12121212`

Your server must return `200 OK` with a JSON body within 10 seconds.

---

## Firewall / IP Allowlist

If your server is behind a firewall, query Agora's NCS IP list and allowlist all returned IPs. Refresh at least **every 24 hours**.

```
GET https://api.agora.io/v2/ncs/ip
Authorization: Basic <Base64(customerID:customerSecret)>
```

Response:

```json
{
  "data": {
    "service": {
      "hosts": [
        { "primaryIP": "xxx.xxx.xxx.xxx" },
        { "primaryIP": "xxx.xxx.xxx.xxx" }
      ]
    }
  }
}
```

---

## DeltaCast Integration Notes

- **Endpoints**: `/v1/webhook/agora/channel` (RTC Channel Events), `/v1/webhook/agora/media-push` (Media Push NCS)
- **Key event**: `103` (broadcaster join) → triggers Media Push relay start to GCP and YouTube.
- **Key event**: `104` (broadcaster leave) → can be used to trigger relay stop.
- Signature verified with `AGORA_CHANNEL_NCS_SECRET` (V1 HMAC/SHA1).
- No JWT auth on webhook endpoints — security is via signature only.
- State machine in `LiveService` acts as the primary idempotency guard.
- `clientSeq` is tracked in `Session.LastBroadcasterClientSeq` to deduplicate event-103 replays.
- Channel name validation ensures events for `test_webhook` (health check) are safely ignored.
