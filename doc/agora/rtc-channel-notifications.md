# Agora Notifications — Receive Channel Event Notifications

Source: https://docs.agora.io/en/interactive-live-streaming/advanced-features/receive-notifications

---

## Overview

Agora Notification Center Service (NCS) sends HTTPS POST callbacks to your webhook when subscribed RTC channel events occur. Your server must authenticate the notification and return `200 OK` within **10 seconds**. The response body must be in **JSON format**.

Retry policy: If `200 OK` is not received within 10 s, Agora immediately resends. The interval between retries increases gradually. Notifications stops after **3 retries**.

---

## Request Format

### HTTP Method

`POST` (HTTPS only — HTTP is not supported)

### Headers

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/json` |
| `Agora-Signature` | HMAC/SHA1 signature computed from the NCS secret + raw request body. |
| `Agora-Signature-V2` | HMAC/SHA256 signature computed from the NCS secret + raw request body. |

### Body Fields

| Field | Type | Description |
|-------|------|-------------|
| `noticeId` | String | Unique ID for this notification callback. |
| `productId` | Number | Product ID: `1`=RTC, `3`=Cloud Recording, `4`=Media Pull, `5`=Media Push |
| `eventType` | Number | Type of event (see Event Types below). |
| `notifyMs` | Number | Unix timestamp (ms) when NCS sends the callback. Updated on resend. |
| `payload` | JSON Object | Event-specific payload (varies per event type). |

#### Example

```json
{
  "noticeId": "2000001428:4330:107",
  "productId": 1,
  "eventType": 101,
  "notifyMs": 1611566412672,
  "payload": { "..." : "..." }
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
func calcSignature(secret, payload string) string {
    mac := hmac.New(sha1.New, []byte(secret))
    mac.Write([]byte(payload))
    return hex.EncodeToString(mac.Sum(nil))
}

// Use hmac.Equal for constant-time comparison (timing-attack safe).
func verifySignature(requestBody, signature string) bool {
    expected := calcSignature(secret, requestBody)
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

---

## Response Requirements

- HTTP status: **`200 OK`**
- Body: **JSON format** (e.g., `{"status": "ok"}`)
- Must respond within **10 seconds**
- Best practice: enable HTTP keep-alive (`MaxKeepAliveRequests ≥ 100`, `KeepAliveTimeout ≥ 10s`)

---

## Event Types (RTC Channel Events)

| Code | Name | Description |
|------|------|-------------|
| `101` | channel create | First user joins; channel initialized. |
| `102` | channel destroy | Last user leaves; channel destroyed. |
| `103` | broadcaster join channel | Host joins in streaming (LIVE_BROADCASTING) profile. |
| `104` | broadcaster leave channel | Host leaves in streaming profile. |
| `105` | audience join channel | Audience member joins in streaming profile. |
| `106` | audience leave channel | Audience member leaves in streaming profile. |
| `107` | user join (communication) | User joins in communication profile. |
| `108` | user leave (communication) | User leaves in communication profile. |
| `111` | role → broadcaster | Audience switches to host role. |
| `112` | role → audience | Host switches to audience role. |

### Recommended subscriptions

- **LIVE_BROADCASTING profile**: `103`, `104`, `105`, `106`, `111`, `112`
- **COMMUNICATION profile**: `103`, `104`, `111`, `112`

### Common Payload Fields (events 103–112)

| Field | Type | Description |
|-------|------|-------------|
| `channelName` | String | Channel name. |
| `uid` | Number | User ID. |
| `platform` | Number | `1`=Android, `2`=iOS, `5`=Windows, `6`=Linux, `7`=Web, `8`=macOS, `0`=Other |
| `clientType` | Number | Linux only (`platform=6`): `3`=On-premise Recording, `10`=Cloud Recording |
| `clientSeq` | Number (UInt64) | Sequence number; monotonically increasing per user. Use to order/deduplicate events. |
| `ts` | Number | Unix timestamp (s) when the event occurred on the RTC server. |
| `reason` | Number | (Leave events only) Why the user left — see Reason Codes below. |
| `duration` | Number | (Leave events only) Time (s) the user spent in the channel. |

### Events 101 / 102 Payload

| Field | Type | Description |
|-------|------|-------------|
| `channelName` | String | Channel name. |
| `ts` | Number | Unix timestamp (s). |
| `lastUid` | Number | (102 only) Last user to leave. |

### Reason Codes (leave events)

| Code | Meaning |
|------|---------|
| `1` | User quit normally. |
| `2` | Connection timeout (no data for >10 s). |
| `3` | Permission — kicked via Banning API. |
| `4` | Agora RTC server internal issue (transient). |
| `5` | User joined from a new device; old device forced out. |
| `9` | SDK disconnected due to multiple client IPs. |
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

- **Endpoint**: `/v1/webhook/agora/channel` (RTC Channel Events), `/v1/webhook/agora/media-push` (Media Push NCS)
- **Key event**: `103` (broadcaster join) → triggers Media Push relay start.
- **Key event**: `104` (broadcaster leave) → can be used to trigger relay stop.
- Signature verified with `AGORA_CHANNEL_NCS_SECRET` (V1 HMAC/SHA1).
- No JWT auth on webhook endpoints — security is via signature only.
- State machine in `LiveService` acts as the primary idempotency guard (duplicate `103` events are safe as long as state is already `live`).
- `clientSeq` is available in the channel event payload and can be used to prevent duplicate `103` triggers if the service ever relaxes state machine strictness.
