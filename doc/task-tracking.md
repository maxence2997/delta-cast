# 項目追蹤 (Task Tracking)

## Phase 1: 基礎設施與環境配置

- [ ] **Agora**: 建立專案、獲取 App ID/Certificate 並配置 REST API 存取權限。
- [ ] **YouTube**: 啟用 Data API v3，完成測試頻道直播功能驗證 (24h 審核)。
- [ ] **GCP**: 啟用 Live Stream API，配置 GCS Bucket (CORS/Public Access) 與 Cloud CDN。

## Phase 2: Golang 後端開發 (Orchestrator)

- [x] **JWT Auth Middleware**: 實作 HS256 JWT Bearer Token 驗證中介層。
- [x] **API Provider**: 封裝 Agora (Token/Media Push), GCP (Live Stream API), YouTube (Stream Key)。
- [x] **Agora Webhook**: 實作 `POST /v1/webhook/agora`，接收 Agora NCS 事件並觸發 Media Push。
- [x] **Live Service**: 實作 Prepare/Start/Stop 邏輯調度，管理 Session 狀態機與流媒體生命週期。
- [x] **Agora Media Push 無轉碼直推模式**: 將 Agora Media Push 預設改為不轉碼直接推流（raw relay），透過 `AGORA_TRANSCODING_ENABLED` 環境變數可選用轉碼，降低 POC 費用。
- [x] **Endpoints**: 完成 `POST /v1/live/prepare`、`POST /v1/live/start`、`POST /v1/live/stop`、`GET /v1/live/status`。

## Phase 3: 前端開發與驗證

- [x] **Web Streamer**: 實作 Agora Web SDK 推流介面。
- [x] **Web Audience**: 實作 Video.js 播放器，串接 Cloud CDN HLS URL。
- [ ] **Integration Test**: 驗證一鍵開播後，YouTube 與 Web 播放器同步顯示畫面。

## Phase 4: Nice to Have (擴充項目)

- [ ] **Mobile Support**: iOS/Android 引入 YouTube 插件觀看。
- [ ] **Native Player**: iOS/Android 串接 GCP HLS 播放器。
- [ ] **Mobile Streamer**: 實作行動端 Agora 開播功能。
- [ ] **Cross-Platform SDKs**:
  - [ ] Web Live SDK 封裝
  - [ ] Android Live SDK 封裝
  - [ ] iOS Live SDK 封裝
- [ ] **Media CDN**: 測試將 Cloud CDN 替換為 Media CDN 的延遲表現。
