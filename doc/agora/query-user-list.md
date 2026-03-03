# Query User List — Quick Reference

> **Source**: [Agora Docs – Query user list](https://docs.agora.io/en/interactive-live-streaming/channel-management-api/endpoint/query-channel-information/query-user-list)

---

## Endpoint

```
GET https://api.agora.io/dev/v1/channel/user/{appid}/{channelName}
```

Optional query parameter: `hosts_only` (when present, returns only the host list in LIVE_BROADCASTING channels)

## Authentication

```
Authorization: Basic <Base64(CustomerID:CustomerSecret)>
```

## Path Parameters

| Parameter     | Required | Description                                                |
| ------------- | -------- | ---------------------------------------------------------- |
| `appid`       | ✓        | Agora App ID of the project                                |
| `channelName` | ✓        | The channel name                                           |
| `hosts_only`  | —        | Optional; when set, returns hosts only (LIVE_BROADCASTING) |

## Response Fields

| Field                 | Description                                                             |
| --------------------- | ----------------------------------------------------------------------- |
| `success`             | Boolean — whether the request succeeded                                 |
| `data.channel_exist`  | Boolean — whether the channel exists; other fields omitted when `false` |
| `data.mode`           | `1` = COMMUNICATION, `2` = LIVE_BROADCASTING                            |
| `data.total`          | Total user count (mode=1 only)                                          |
| `data.users`          | Array of all user IDs (mode=1 only)                                     |
| `data.broadcasters`   | Array of host user IDs (mode=2 only)                                    |
| `data.audience`       | Array of up to 10,000 audience user IDs (mode=2, without `hosts_only`)  |
| `data.audience_total` | Total audience count (mode=2 only)                                      |

## Response Examples

**COMMUNICATION (mode=1)**

```json
{
  "success": true,
  "data": {
    "channel_exist": true,
    "mode": 1,
    "total": 1,
    "users": [906218805]
  }
}
```

**LIVE_BROADCASTING (mode=2)**

```json
{
  "success": true,
  "data": {
    "channel_exist": true,
    "mode": 2,
    "broadcasters": [2206227541, 2845863044],
    "audience": [906219905],
    "audience_total": 1
  }
}
```

**Channel does not exist**

```json
{
  "success": true,
  "data": {
    "channel_exist": false
  }
}
```
