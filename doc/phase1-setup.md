# Phase 1: 基礎設施設定指南

本文件為各服務設定的**索引頁**。各服務詳細步驟已移至獨立文件。

> **預估時間**：約 1–2 小時（YouTube 直播功能審核需額外等待 24 小時）。

---

## 各服務設定文件

| 服務    | 文件                                                 | 取得的環境變數                                                                                 |
| ------- | ---------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Agora   | [doc/setup/agora-setup.md](setup/agora-setup.md)     | `AGORA_APP_ID` `AGORA_APP_CERTIFICATE` `AGORA_REST_KEY` `AGORA_REST_SECRET` `AGORA_NCS_SECRET` |
| YouTube | [doc/setup/youtube-setup.md](setup/youtube-setup.md) | `YOUTUBE_CLIENT_ID` `YOUTUBE_CLIENT_SECRET` `YOUTUBE_REFRESH_TOKEN`                            |
| GCP     | [doc/setup/gcp-setup.md](setup/gcp-setup.md)         | `GCP_PROJECT_ID` `GCP_REGION` `GCP_BUCKET_NAME` `GCP_CDN_DOMAIN`                               |

---

## 相關腳本

| 腳本                       | 用途                          |
| -------------------------- | ----------------------------- |
| `scripts/gcp-setup.sh`     | 一鍵建立所有 GCP 靜態資源     |
| `scripts/gcp-teardown.sh`  | 一鍵清除所有 GCP 靜態資源     |
| `scripts/gcp-cdn-armor.sh` | 管理 Cloud Armor CDN 防護規則 |

---

## 驗證清單

全部完成後，確認 `server/.env.local` 中以下變數都已填入真實值：

```
✅ AGORA_APP_ID
✅ AGORA_APP_CERTIFICATE
✅ AGORA_REST_KEY
✅ AGORA_REST_SECRET
✅ AGORA_NCS_SECRET
✅ GCP_PROJECT_ID
✅ GCP_REGION
✅ GCP_BUCKET_NAME
✅ GCP_CDN_DOMAIN
✅ YOUTUBE_CLIENT_ID
✅ YOUTUBE_CLIENT_SECRET
✅ YOUTUBE_REFRESH_TOKEN
✅ GOOGLE_APPLICATION_CREDENTIALS（系統環境變數，指向 ~/deltacast-sa-key.json）
```

全部通過後，Phase 1 完成！回到 `task-tracking.md` 勾選三個項目，並開始 Phase 2 開發。
