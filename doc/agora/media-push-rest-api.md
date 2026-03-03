# Agora Media Push — RESTful API Quick Reference

> **Source**: <https://docs.agora.io/en/media-push/develop/restful-api>

---

## Authentication

```
Authorization: Basic <Base64(CustomerID:CustomerSecret)>
```

## Base URL

```
https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters
```

`region`: `cn` | `ap` | `na` | `eu` — must match the CDN origin location.

---

## Create Converter (Non-Transcoded)

```
POST https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters
```

**Request Body**

```json
{
  "converter": {
    "name": "deltacast-<sessionId>_gcp",
    "rawOptions": {
      "rtcChannel": "<agoraChannel>",
      "rtcStreamUid": 0
    },
    "rtmpUrl": "rtmp://<destination>",
    "idleTimeOut": 300,
    "jitterBufferSizeMs": 1000
  }
}
```

| Field                     | Required | Description                                             |
| ------------------------- | -------- | ------------------------------------------------------- |
| `rawOptions.rtcChannel`   | ✓        | Agora channel name (max 64 chars)                       |
| `rawOptions.rtcStreamUid` | ✓        | UID of the source stream                                |
| `rtmpUrl`                 | ✓        | RTMP push destination (max 1024 chars)                  |
| `name`                    | —        | Optional; unique per project; recommended for dedup     |
| `idleTimeOut`             | —        | Seconds before auto-destroy when channel is empty       |
| `jitterBufferSizeMs`      | —        | `[0, 1000]`, default `1000`; `0` disables (not advised) |

**Response `2XX`**

```json
{
  "converter": {
    "id": "4c014467d647bb87b60b719f6fa57686",
    "createTs": 1591786766,
    "updateTs": 1591786766,
    "state": "connecting"
  },
  "fields": "id,createTs,updateTs,state"
}
```

> Save the `converter.id` — needed for Delete and status tracking.

---

## Delete Converter

```
DELETE https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters/{converterId}
```

**Response**: `2XX` with empty body on success.

---

## HTTP Status Codes

| Code  | Meaning             | Notes                                           |
| ----- | ------------------- | ----------------------------------------------- |
| `200` | OK                  |                                                 |
| `400` | Bad Request         | Invalid `rtmpUrl` or `idleTimeout`              |
| `401` | Unauthorized        | Invalid credentials                             |
| `403` | Forbidden           | Project not authorized for Media Push           |
| `404` | Not Found           | Converter not found or already destroyed        |
| `409` | Conflict            | A Converter with the same `name` already exists |
| `429` | Too Many Requests   | Rate limit exceeded                             |
| `503` | Service Unavailable | Retry with backoff                              |
| `504` | Gateway Timeout     | Check if resource was created; re-create if not |

---

## DeltaCast Integration Notes

- DeltaCast uses **non-transcoded mode** (`rawOptions`) — see `spec.md` § 4.1 for rationale.
- Two Converters are created per session: one targeting GCP RTMP Input URI, one targeting YouTube RTMP URL+Key.
- Converter IDs are stored in `session.MediaPushGCPSID` / `session.MediaPushYouTubeSID` and used in `Stop()`.
- Provider: `server/internal/provider/agora_media_push.go`
