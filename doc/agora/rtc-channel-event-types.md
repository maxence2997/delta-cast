# Agora RTC Channel Event Types — Webhook Reference

> Source: <https://docs.agora.io/en/interactive-live-streaming/channel-management-api/webhook/channel-event-type>

---

## Overview

When the Agora Notifications service is enabled, the Agora server sends channel event callbacks to
your webhook as HTTPS POST requests. The data format is JSON, character encoding is UTF-8, and the
signature algorithm is HMAC/SHA1 or HMAC/SHA256.

This webhook is configured under **Agora Console → Notifications → RTC Channel Event Callbacks**.
The secret shown there is `AGORA_CHANNEL_NCS_SECRET` in DeltaCast.

---

## Request Headers

| Header               | Description                                                                                |
| -------------------- | ------------------------------------------------------------------------------------------ |
| `Content-Type`       | `application/json`                                                                         |
| `Agora-Signature`    | HMAC/SHA1 signature computed from the raw request body using the RTC Channel NCS secret.   |
| `Agora-Signature-V2` | HMAC/SHA256 signature computed from the raw request body using the RTC Channel NCS secret. |

> DeltaCast verifies `Agora-Signature` (HMAC/SHA1).

---

## Request Body Fields

| Field       | Type        | Description                                                                                  |
| ----------- | ----------- | -------------------------------------------------------------------------------------------- |
| `noticeId`  | String      | Notification ID. Identifies a single event notification. Use with `notifyMs` to deduplicate. |
| `productId` | Number      | Business ID. Always `1` for RTC Channel events.                                              |
| `eventType` | Number      | The type of event. See [Channel Event Types](#channel-event-types) below.                    |
| `notifyMs`  | Number      | Unix timestamp (ms) when the notification was sent. Updated on retries.                      |
| `sid`       | String      | Session ID.                                                                                  |
| `payload`   | JSON Object | Event-specific content. See each event type for fields.                                      |

### Request Body Example

```json
{
  "noticeId": "2000001428:4330:107",
  "productId": 1,
  "eventType": 103,
  "notifyMs": 1611566412672,
  "payload": { "..." }
}
```

---

## Channel Event Types

| Code | Name                               | Description                                                     |
| ---- | ---------------------------------- | --------------------------------------------------------------- |
| 101  | channel create                     | The first user joins — the channel is created.                  |
| 102  | channel destroy                    | The last user leaves — the channel is destroyed.                |
| 103  | broadcaster join channel           | A host joins the channel (live broadcast profile).              |
| 104  | broadcaster leave channel          | A host leaves the channel (live broadcast profile).             |
| 105  | audience join channel              | An audience member joins the channel (live broadcast profile).  |
| 106  | audience leave channel             | An audience member leaves the channel (live broadcast profile). |
| 107  | user join channel (communication)  | A user joins the channel (communication profile).               |
| 108  | user leave channel (communication) | A user leaves the channel (communication profile).              |
| 111  | client role change to broadcaster  | An audience member switches role to host.                       |
| 112  | client role change to audience     | A host switches role to audience member.                        |

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

| Field         | Type   | Description                                                                                     |
| ------------- | ------ | ----------------------------------------------------------------------------------------------- |
| `channelName` | String | The channel name.                                                                               |
| `ts`          | Number | Unix timestamp (s) when the event occurred.                                                     |
| `lastUid`     | Number | The UID of the last user to leave. Multiple callbacks may appear if users leave simultaneously. |

```json
{ "channelName": "test_webhook", "ts": 1560399999, "lastUid": 12121212 }
```

---

### 103 — broadcaster join channel

| Field         | Type   | Description                                                                                           |
| ------------- | ------ | ----------------------------------------------------------------------------------------------------- |
| `channelName` | String | The channel name.                                                                                     |
| `uid`         | Number | The host's UID.                                                                                       |
| `platform`    | Number | Device platform: 1=Android, 2=iOS, 5=Windows, 6=Linux, 7=Web, 8=macOS, 0=Other.                       |
| `clientType`  | Number | Returned only if `platform=6`. Service type: 3=Local server recording, 8=Applets, 10=Cloud recording. |
| `clientSeq`   | Number | Serial number for ordering events from the same user (client-side timestamp).                         |
| `ts`          | Number | Unix timestamp (s) when the event occurred on the business server.                                    |
| `account`     | String | The user account / user ID string.                                                                    |

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

> **DeltaCast usage:** This event (`eventType=103`) triggers Media Push to GCP and YouTube when the streamer joins.

---

### 104 — broadcaster leave channel

| Field         | Type   | Description                                                                                          |
| ------------- | ------ | ---------------------------------------------------------------------------------------------------- |
| `channelName` | String | The channel name.                                                                                    |
| `uid`         | Number | The host's UID.                                                                                      |
| `platform`    | Number | Device platform (same codes as event 103).                                                           |
| `clientType`  | Number | Returned only if `platform=6`.                                                                       |
| `clientSeq`   | Number | Serial number for ordering events from the same user.                                                |
| `reason`      | Number | Leave reason: 1=Normal, 2=Timeout, 3=Permission, 4=Server internal, 5=Device switch, 9=Multiple IPs. |
| `ts`          | Number | Unix timestamp (s) when the event occurred.                                                          |
| `duration`    | Number | Time the user spent in the channel (seconds).                                                        |
| `account`     | String | The user account / user ID string.                                                                   |

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

Same fields as [broadcaster join channel](#103--broadcaster-join-channel), with `uid` being the audience member's UID.

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

Same fields as [broadcaster leave channel](#104--broadcaster-leave-channel), with `uid` being the audience member's UID.

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

| Field         | Type   | Description                                                        |
| ------------- | ------ | ------------------------------------------------------------------ |
| `channelName` | String | The channel name.                                                  |
| `uid`         | Number | The user's UID.                                                    |
| `clientSeq`   | Number | Serial number for ordering events from the same user.              |
| `ts`          | Number | Unix timestamp (s) when the event occurred on the business server. |
| `account`     | String | The user account / user ID string.                                 |

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

## Signature Verification

Agora signs the raw POST body using HMAC/SHA1 with the **RTC Channel NCS secret** (`AGORA_CHANNEL_NCS_SECRET`):

```go
mac := hmac.New(sha1.New, []byte(secret))
mac.Write(body)
signature := hex.EncodeToString(mac.Sum(nil))
// compare with Agora-Signature header
```

> For higher security, use `Agora-Signature-V2` (HMAC/SHA256) instead.

---

## Reliability Considerations

- **Out-of-order delivery**: Notifications are not guaranteed to arrive in event order. Use `clientSeq` (client-side) or `ts` (server-side) to sort events for the same user.
- **Duplicate delivery**: Agora may send the same notification more than once. Use `noticeId` + `notifyMs` together to deduplicate.
- **Retries**: If your server does not respond with `200 OK` within 10 seconds, Agora retries up to 3 times with increasing intervals.
- **Response format**: Response body must be valid JSON.
