# Agora Media Push — RESTful API Reference

> Source: <https://docs.agora.io/en/media-push/develop/restful-api>

---

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Base URL & Region](#base-url--region)
- [Common Request Headers](#common-request-headers)
- [Create Converter](#create-converter)
- [Delete Converter](#delete-converter)
- [Update Converter](#update-converter)
- [Get Converter Status](#get-converter-status)
- [List Converters](#list-converters)
- [Rate Limits](#rate-limits)
- [Status Codes](#status-codes)
- [Recommended Video Profiles](#recommended-video-profiles)
- [Considerations](#considerations)

---

## Overview

Media Push processes a media stream in an Agora RTC channel and pushes it to a CDN via RTMP. The processing unit is called a **Converter**.

| Operation  | Description                                                                           |
| ---------- | ------------------------------------------------------------------------------------- |
| **Create** | Create a Converter; configure transcoded or non-transcoded streaming and push to CDN. |
| **Delete** | Destroy a Converter; stop pushing the stream.                                         |
| **Update** | Update a running Converter's configuration (layout, RTMP URL, etc.).                  |
| **Get**    | Query the current status of a specific Converter.                                     |
| **List**   | Query all Converters under a project or a specific channel.                           |

Two modes are available:

- **Without transcoding** — forward a single user's stream as-is (single-host). Uses `rawOptions`.
- **With transcoding** — mix multiple audio/video streams (multi-host). Uses `transcodeOptions`.

> DeltaCast primarily uses **non-transcoded** mode (single streamer → YouTube RTMP).

---

## Authentication

All requests require **HTTP Basic Auth**.

```
Authorization: Basic <base64(customer_id:customer_secret)>
```

See [Agora RESTful API Authentication](https://docs.agora.io/en/media-push/reference/restful-authentication) for details on obtaining credentials.

---

## Base URL & Region

```
https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters
```

| Path Param | Type   | Description                                                                    |
| ---------- | ------ | ------------------------------------------------------------------------------ |
| `region`   | String | Region where the Converter is created. **Must match** the CDN origin location. |
| `appId`    | String | Agora App ID (from Agora Console).                                             |

Supported `region` values (lowercase only):

| Value | Region                          |
| ----- | ------------------------------- |
| `cn`  | China Mainland                  |
| `ap`  | Asia (excluding Mainland China) |
| `na`  | North America                   |
| `eu`  | Europe                          |

> **TLS required** — only HTTPS with TLS 1.0/1.1/1.2 is supported.

---

## Common Request Headers

| Header          | Value / Description                                                                                                                                                    |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Content-Type`  | `application/json`                                                                                                                                                     |
| `Authorization` | Basic HTTP auth (see [Authentication](#authentication)).                                                                                                               |
| `X-Request-ID`  | UUID identifying the request. Agora echoes it back as `X-Request-ID` in response and returns `X-Custom-Request-ID` for troubleshooting. **Recommended** to always set. |

---

## Create Converter

### Request

```
POST https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters?regionHintIp={regionHintIp}
```

| Query Param    | Type   | Required | Description                                                         |
| -------------- | ------ | -------- | ------------------------------------------------------------------- |
| `regionHintIp` | String | No       | IPv4 address of the CDN origin. Helps ensure RTMP stream stability. |

### Request Body — Non-Transcoded (Single Host)

```jsonc
{
  "converter": {
    "name": "show68_raw",                     // Optional, max 64 chars, unique per project
    "rawOptions": {
      "rtcChannel": "show68",                 // Required — Agora channel name
      "rtcStreamUid": 201                     // Required — UID of the source stream
    },
    "rtmpUrl": "rtmp://a.rtmp.youtube.com/live2/xxxx-xxxx",  // Required, max 1024 chars
    "idleTimeOut": 300,                       // Optional — auto-destroy after N seconds idle
    "jitterBufferSizeMs": 1000                // Optional — [0, 1000], rounded to nearest 100
  }
}
```

#### Non-Transcoded Fields

| Field                     | Type   | Required | Description                                                                                                                         |
| ------------------------- | ------ | -------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `name`                    | String | No       | Converter name. Max 64 chars. Must be unique within project. Agora recommends combining channel name + feature (e.g. `show68_raw`). |
| `rawOptions`              | Object | **Yes**  | Non-transcoded push configuration.                                                                                                  |
| `rawOptions.rtcChannel`   | String | **Yes**  | Agora channel name (max 64 chars).                                                                                                  |
| `rawOptions.rtcStreamUid` | Number | **Yes**  | UID of the media stream to forward.                                                                                                 |
| `rtmpUrl`                 | String | **Yes**  | RTMP push address (max 1024 chars).                                                                                                 |
| `idleTimeOut`             | Number | No       | Max idle seconds before auto-destroy. Idle = all users left the channel.                                                            |
| `jitterBufferSizeMs`      | Number | No       | Jitter buffer delay in ms. Default `1000`. Range `[0, 1000]`, rounded to nearest 100. `0` disables jitter buffer (not recommended). |

### Request Body — Transcoded (Multi-Host)

```jsonc
{
  "converter": {
    "name": "show68_vertical",
    "transcodeOptions": {
      "rtcChannel": "show68",
      "audioOptions": {
        "codecProfile": "LC-AAC",        // or "HE-AAC"
        "sampleRate": 48000,
        "bitrate": 48,                   // Kbps
        "audioChannels": 1,
        "rtcStreamUids": [201, 202]      // Optional — omit to mix all users
      },
      "videoOptions": {
        "canvas": {
          "width": 360,
          "height": 640,
          "color": 0                     // Background color (optional)
        },
        "layout": [                      // Required when layoutType is 0 or empty
          {
            "rtcStreamUid": 201,
            "region": { "xPos": 0, "yPos": 0, "zIndex": 1, "width": 360, "height": 320 },
            "fillMode": "fill",
            "placeholderImageUrl": "http://example.com/placeholder.jpg"
          },
          {
            "rtcStreamUid": 202,
            "region": { "xPos": 0, "yPos": 320, "zIndex": 1, "width": 360, "height": 320 }
          }
        ],
        "layoutType": 0,                // 0 = custom (default), 1 = vertical
        "codec": "H.264",               // or "H.265"
        "codecProfile": "High",         // "High" | "baseline" | "main"
        "frameRate": 15,                // fps [1, 30], default 15
        "gop": 30,                      // default = frameRate * 2
        "bitrate": 400,                // Kbps [1, 10000]
        "seiOptions": {},
        "defaultPlaceholderImageUrl": "http://example.com/default.jpg"
      }
    },
    "rtmpUrl": "rtmp://example/live/show68",
    "idleTimeOut": 300,
    "jitterBufferSizeMs": 1000
  }
}
```

#### Key Transcoded Fields

| Field                                                      | Type     | Required    | Description                                                                                       |
| ---------------------------------------------------------- | -------- | ----------- | ------------------------------------------------------------------------------------------------- |
| `transcodeOptions`                                         | Object   | **Yes**     | Transcoding configuration.                                                                        |
| `transcodeOptions.rtcChannel`                              | String   | **Yes**     | Agora channel name (max 64 chars).                                                                |
| `transcodeOptions.audioOptions`                            | Object   | No*         | Audio config. Required for audio+video; omit for video-only. Can be empty `{}` for defaults.      |
| `transcodeOptions.audioOptions.codecProfile`               | String   | No          | `"LC-AAC"` or `"HE-AAC"`.                                                                         |
| `transcodeOptions.audioOptions.sampleRate`                 | Number   | No          | Audio sample rate (e.g. `48000`).                                                                 |
| `transcodeOptions.audioOptions.bitrate`                    | Number   | No          | Audio bitrate in Kbps.                                                                            |
| `transcodeOptions.audioOptions.audioChannels`              | Number   | No          | Number of audio channels (1 or 2).                                                                |
| `transcodeOptions.audioOptions.rtcStreamUids`              | Number[] | No          | UIDs to mix. Omit to mix **all** users.                                                           |
| `transcodeOptions.videoOptions`                            | Object   | No*         | Video config. Required for audio+video; omit for audio-only. **Cannot be empty**.                 |
| `transcodeOptions.videoOptions.canvas`                     | Object   | **Yes**     | Canvas dimensions (`width`, `height`, optional `color`).                                          |
| `transcodeOptions.videoOptions.layout`                     | Array    | Conditional | Required when `layoutType` is `0` or empty. Array of `RtcStreamView` / `ImageView` elements.      |
| `transcodeOptions.videoOptions.layoutType`                 | Number   | No          | `0` = custom (default), `1` = vertical.                                                           |
| `transcodeOptions.videoOptions.codec`                      | String   | No          | `"H.264"` (default) or `"H.265"`.                                                                 |
| `transcodeOptions.videoOptions.codecProfile`               | String   | No          | `"High"` (default), `"baseline"`, or `"main"`. Auto-set to `"main"` when codec is H.265.          |
| `transcodeOptions.videoOptions.frameRate`                  | Number   | No          | fps `[1, 30]`, default `15`.                                                                      |
| `transcodeOptions.videoOptions.gop`                        | Number   | No          | Default = `frameRate * 2`.                                                                        |
| `transcodeOptions.videoOptions.bitrate`                    | Number   | **Yes**     | Kbps `[1, 10000]`.                                                                                |
| `transcodeOptions.videoOptions.seiOptions`                 | Object   | No          | SEI info in output stream. Default empty (no SEI).                                                |
| `transcodeOptions.videoOptions.defaultPlaceholderImageUrl` | String   | No          | Background image when a user stops publishing. Supports JPG/PNG/GIF.                              |
| `transcodeOptions.videoOptions.vertical`                   | Object   | No          | Required when `layoutType` is `1`. Contains `maxResolutionUid`, `fillMode`, `refreshIntervalSec`. |

### Response

#### Response Headers

| Header          | Description                             |
| --------------- | --------------------------------------- |
| `X-Request-ID`  | Echoed request UUID.                    |
| `X-Resource-ID` | UUID identifying the created Converter. |

#### Response Body (2XX)

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

| Field      | Type   | Description                                      |
| ---------- | ------ | ------------------------------------------------ |
| `id`       | String | Converter UUID assigned by Agora.                |
| `createTs` | Number | Unix timestamp (seconds) of creation.            |
| `updateTs` | Number | Unix timestamp (seconds) of last update.         |
| `state`    | String | `"connecting"` · `"running"` · `"failed"`        |
| `fields`   | String | Field mask indicating which fields are returned. |

#### Error Response

```json
{ "message": "Invalid authentication credentials." }
```

---

## Delete Converter

### Request

```
DELETE https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters/{converterId}
```

| Path Param    | Type   | Description                            |
| ------------- | ------ | -------------------------------------- |
| `converterId` | String | The Converter ID returned from Create. |

### Response

- **2XX** — success, body is **empty**.
- **Non-2XX** — body contains `message` string.

---

## Update Converter

### Request

```
PATCH https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters/{converterId}?sequence={sequence}
```

| Query Param | Type   | Required | Description                                                                                                                                     |
| ----------- | ------ | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `sequence`  | Number | **Yes**  | Sequential request number (≥ 0). Each subsequent Update must use a **higher** value. Agora applies the update with the largest sequence number. |

### Request Body

Same structure as the Create request body, wrapped in `converter` and `fields`.

For **non-transcoded** Converters, the primary updatable field is `rtmpUrl`:

```json
{
  "converter": {
    "rtmpUrl": "rtmp://a.rtmp.youtube.com/live2/new-stream-key"
  },
  "fields": "rtmpUrl"
}
```

For **transcoded** Converters, layout and video options can also be updated:

```json
{
  "converter": {
    "transcodeOptions": {
      "videoOptions": {
        "canvas": { "width": 360, "height": 640, "color": 0 },
        "layout": [
          {
            "rtcStreamUid": 201,
            "region": { "xPos": 0, "yPos": 0, "zIndex": 1, "width": 360, "height": 320 },
            "fillMode": "fill",
            "placeholderImageUrl": "http://example/host_placeholder.jpg"
          }
        ]
      }
    }
  },
  "fields": "transcodeOptions.videoOptions.canvas,transcodeOptions.videoOptions.layout"
}
```

### Non-Updatable Fields

The following fields **cannot** be changed via Update:

- `name`
- `idleTimeOut`
- `transcodeOptions.rtcChannel`
- `transcodeOptions.audioOptions` (and all sub-fields: `codecProfile`, `sampleRate`, `bitrate`, `audioChannels`)
- `transcodeOptions.videoOptions.codec`
- `transcodeOptions.videoOptions.codecProfile`

> When using vertical layout (`layoutType: 1`), only `rtmpUrl` can be updated.

### Response Body (2XX)

Same structure as Create response:

```json
{
  "converter": {
    "id": "4c014467d647bb87b60b719f6fa57686",
    "createTs": 1591786766,
    "updateTs": 1591788746,
    "state": "running"
  },
  "fields": "id,createTs,updateTs,state"
}
```

---

## Get Converter Status

### Request

```
GET https://api.agora.io/{region}/v1/projects/{appId}/rtmp-converters/{converterId}
```

### Response Body (2XX) — Non-Transcoded

```json
{
  "converter": {
    "name": "show68_raw",
    "rawOptions": {
      "rtcChannel": "show68",
      "rtcStreamUid": 201
    },
    "rtmpUrl": "rtmp://a.rtmp.youtube.com/live2/xxxx",
    "idleTimeout": 120
  }
}
```

### Response Body (2XX) — Transcoded

Returns full `transcodeOptions` including `audioOptions`, `videoOptions`, `layout`, etc., plus `rtmpUrl`, `idleTimeout`, `createTs`, `updateTs`, `state`.

### Destroyed Converter

```json
{ "reason": "Resource is not found and destroyed." }
```

---

## List Converters

### Request

Query all Converters under a project:

```
GET https://api.agora.io/v1/projects/{appId}/rtmp-converters?cursor={cursor}
```

Query Converters for a specific channel:

```
GET https://api.agora.io/v1/projects/{appId}/channels/{cname}/rtmp-converters?cursor={cursor}
```

| Query Param | Type   | Required | Description                                                                                                                                      |
| ----------- | ------ | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `cursor`    | String | No       | Pagination cursor. Omit on first request. When response `cursor` is `0`, all results have been returned. Each page returns up to 500 Converters. |

### Response Body (2XX)

```json
{
  "success": true,
  "data": {
    "total_count": 1,
    "cursor": 0,
    "members": [
      {
        "rtcChannel": "testchannel",
        "status": "200",
        "converterName": "show68_raw",
        "updateTs": "1641267823",
        "appId": "abc123xxx",
        "rtmpUrl": "rtmp://example/live/areu",
        "converterId": "889B6D4BEC4XXXE68CCDA978BF21350",
        "create": "1641267818",
        "idleTimeout": "120",
        "state": "running"
      }
    ]
  }
}
```

| Field                          | Type   | Description                               |
| ------------------------------ | ------ | ----------------------------------------- |
| `data.total_count`             | String | Total number of Converters.               |
| `data.cursor`                  | Number | Pagination cursor. `0` = no more pages.   |
| `data.members[].converterId`   | String | Converter UUID.                           |
| `data.members[].converterName` | String | Converter name.                           |
| `data.members[].rtcChannel`    | String | Agora channel name.                       |
| `data.members[].rtmpUrl`       | String | RTMP push URL.                            |
| `data.members[].state`         | String | `"connecting"` · `"running"` · `"failed"` |
| `data.members[].create`        | String | Unix timestamp of creation.               |
| `data.members[].updateTs`      | String | Unix timestamp of last update.            |
| `data.members[].idleTimeout`   | String | Idle timeout in seconds.                  |

---

## Rate Limits

| Operation | Limit                            |
| --------- | -------------------------------- |
| Create    | 50 requests/second per project   |
| Delete    | 50 requests/second per project   |
| Update    | 2 requests/second per Converter  |
| Get       | 50 requests/second per Converter |

Exceeding limits returns `429 Too Many Requests`.

---

## Status Codes

| Code  | Meaning             | Notes                                                         |
| ----- | ------------------- | ------------------------------------------------------------- |
| `200` | OK                  | —                                                             |
| `400` | Bad Request         | Invalid `rtmpUrl` or `idleTimeout`.                           |
| `401` | Unauthorized        | Invalid auth credentials.                                     |
| `403` | Forbidden           | Project not authorized for Media Push. Contact Agora support. |
| `404` | Not Found           | Converter not found or already destroyed.                     |
| `409` | Conflict            | A Converter with the same `name` already exists.              |
| `429` | Too Many Requests   | Rate limit or resource quota exceeded.                        |
| `500` | Internal Error      | Contact Agora support.                                        |
| `501` | Not Implemented     | Method not supported.                                         |
| `503` | Service Unavailable | Retry with backoff.                                           |
| `504` | Gateway Timeout     | Check if resource was created; re-create if not.              |

---

## Recommended Video Profiles

For transcoded mode, Agora recommends these resolution / fps / bitrate combinations:

| Resolution (w × h) | Frame Rate (fps) | Bitrate (Kbps) |
| ------------------ | ---------------- | -------------- |
| 160 × 120          | 15               | 130            |
| 320 × 180          | 15               | 280            |
| 320 × 240          | 15               | 400            |
| 640 × 360          | 15               | 800            |
| 640 × 360          | 30               | 1200           |
| 480 × 360          | 15               | 640            |
| 640 × 480          | 15               | 1000           |
| 640 × 480          | 30               | 1500           |
| 848 × 480          | 15               | 1220           |
| 848 × 480          | 30               | 1860           |
| 1280 × 720         | 15               | 2260           |
| 1280 × 720         | 30               | 3420           |
| 960 × 720          | 15               | 1820           |
| 960 × 720          | 30               | 2760           |

> If you set a bitrate outside a reasonable range, Agora automatically adjusts it.

---

## Considerations

1. **Channel profile** must be set to **live broadcasting**.
2. Always log `X-Request-ID` and `X-Resource-ID` from response headers for troubleshooting.
3. Use the `name` field to avoid creating duplicate Converters. Agora recommends naming as `{channelName}_{feature}`.
4. Set `region` to match the CDN origin location.
5. If a Converter is auto-destroyed due to failure, create a new one.
6. If CDN pull is abnormal after creation, delete the Converter and create a new one.
7. When calling Update multiple times, always increment `sequence`.
8. **Cannot mix transcoding modes** — a Converter is either transcoded or non-transcoded. To switch, create a new Converter.
9. In non-transcoded mode, the Converter **forwards only** (no transcoding). Use H.264 or H.265 at the streaming end for RTMP compatibility. If the source is VP8, use transcoded mode instead.
