# Instruction

## Package Structure

```
delta-cast/
├── doc/                        # 存放 spec.md, instruction.md, task-tracking.md
├── server/                     # Golang (Orchestrator) 專案
│   ├── cmd/                    # 進入點 (main.go)
│   ├── internal/               # 內部模組（不對外暴露）
│   │   ├── handler/            # HTTP handler（路由處理）
│   │   ├── middleware/         # JWT 驗證、logging 等中介層
│   │   ├── service/            # 業務邏輯層（LiveService, Session 管理）
│   │   ├── provider/           # 第三方 API 封裝（Agora, GCP, YouTube）
│   │   ├── model/              # 資料結構定義
│   │   └── config/             # 環境變數載入與配置
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile
│   └── .env.example            # 環境變數範例（不包含敏感資訊）
├── web/                        # 前端 Web 專案 (Next.js / React)
│   ├── src/
│   │   ├── app/                # Next.js App Router 頁面
│   │   ├── components/         # 共用 UI 元件
│   │   └── lib/                # 工具函式與 hooks
│   ├── public/                 # 靜態資源（圖片、字型等）
│   ├── .env.example            # 環境變數範例
│   ├── package.json            # 依賴管理
│   ├── README.md
│   └── ...                     # 其他前端相關配置文件
├── mobile/                     # 未來的 Mobile SDK 或 Demo (Flutter/RN/Native)
│   ├── ios/
│   └── android/
├── shared/                     # 存放 Proto 定義或統一的轉碼參數 JSON（若有）
├── .github/                    # GitHub Actions 自動化腳本
│   └── workflows/
│       ├── deploy-server.yml   # 後端部署腳本
│       ├── deploy-web.yml      # 前端部署腳本
│       ├── deploy-ios.yml      # iOS 部署腳本（未來）
│       └── deploy-android.yml  # Android 部署腳本（未來）
├── docker-compose.yml          # 本地一次性啟動所有服務進行整合測試
└── Makefile                    # 常用命令（如 `make run-server`, `make run-web`, `make test` 等）
```

---

## AI 開發指示

以下規範適用於 AI Agent（如 GitHub Copilot）輔助開發本專案時的行為準則。

### 基本原則

- **先讀再寫**：修改任何程式碼前，必須先閱讀 `doc/spec.md` 瞭解系統設計，再閱讀目標檔案的完整內容。
- **最小變更**：每次修改只處理一個關注點，不做範圍外的重構或「順手改善」。
- **不猜測密鑰**：絕不在程式碼中硬編碼 API Key、Secret 等敏感資訊，一律從環境變數讀取。
- **不自行新增文件紀錄**：除非使用者明確要求，不要建立額外的 markdown 檔案來摘要或紀錄變更。

### 架構規範

- 後端遵循 `handler → service → provider` 三層架構，不可跨層直接呼叫。
- 所有第三方 API 呼叫必須封裝在 `internal/provider/` 內，handler 與 service 層不直接引用第三方 SDK。
- Session 狀態管理集中在 `internal/service/` 層，handler 只做 request/response 轉換。
- 前端元件遵循 Next.js App Router 慣例，頁面放 `app/`，共用元件放 `components/`。

### 程式碼風格

- **Golang**：
  - 遵循 `gofmt` / `goimports` 格式化。
  - 錯誤處理使用標準 `if err != nil` 模式，不使用 panic。
  - Public function 必須附加 GoDoc 註解。
  - 檔名使用 snake_case（如 `live_service.go`）。
- **TypeScript / React**：
  - 使用 ESLint + Prettier 格式化。
  - 元件使用 functional component + hooks，不使用 class component。
  - 檔名使用 kebab-case（如 `live-player.tsx`）。

### Git 規範

- Commit message 使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式：
  - `feat:` 新功能
  - `fix:` 修復
  - `refactor:` 重構
  - `doc:` 文件
  - `chore:` 工具/配置
- 每個 commit 只包含一個邏輯變更，不混合多個不相關修改。
- Commit message 使用英文撰寫。

### 測試要求

- 新增的 service / provider 函數必須附帶單元測試。
- 測試檔案與原始碼放同一目錄，命名加 `_test.go`（Go）或 `.test.ts`（TS）。
- 測試需涵蓋正常路徑與至少一個錯誤路徑。

### 任務追蹤

- 開始多步驟工作前，先建立 todo list 追蹤進度。
- 完成一項任務後立即標記完成，再進入下一項。
- 若任務內容與 `doc/task-tracking.md` 中的 Phase 相關，完成後需同步更新 task-tracking。

---

## 人員開發指示

以下規範適用於人類開發者在本專案中的協作與開發流程。

### 環境需求

| 工具       | 版本需求 | 說明               |
| ---------- | -------- | ------------------ |
| Go         | 1.26+    | 後端開發           |
| Node.js    | 22 LTS+  | 前端開發           |
| pnpm       | 10+      | 前端套件管理       |
| Docker     | 24+      | 本地整合測試       |
| gcloud CLI | latest   | GCP 資源管理與部署 |

### 環境變數設定

1. 複製 `server/.env.example` 到 `server/.env`。
2. 填入所有必要的 API 金鑰與配置。
3. **禁止**將 `.env` 加入版本控制（已在 `.gitignore` 中排除）。

**關鍵選用環境變數：**

| 變數名稱                    | 預設值  | 說明                                                                                                                                                                                                                                              |
| --------------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `AGORA_TRANSCODING_ENABLED` | `false` | 是否啟用 Agora Media Push 轉碼。`false`（預設）為直推模式，不重新編碼原始串流，費用最低，適合 POC 驗證（GCP 與 YouTube 均可直接接收 RTMP 串流）。設為 `true` 可啟用 H.264/AAC 720p 轉碼，適合推流端格式不確定的場景。詳見 `doc/spec.md` 第 4 節。 |

### 本地開發

```bash
# 啟動後端（開發模式）
cd server
go run ./cmd/

# 啟動前端（開發模式）
cd web
pnpm install
pnpm dev

# 一鍵啟動所有服務（Docker）
docker-compose -f docker-compose.local.yml up
```

### 分支策略

- `main`：穩定版本，僅透過 PR 合併。
- `dev`：開發分支，日常開發基於此分支。
- 功能分支命名：`feat/<簡短描述>`（如 `feat/agora-webhook`）。
- 修復分支命名：`fix/<簡短描述>`（如 `fix/stop-cleanup`）。

### Code Review 規範

- 所有合併至 `main` 的 PR 需至少一人 review。
- PR 描述需包含：變更目的、影響範圍、測試方式。
- CI 通過後方可合併。

### 文件維護

- 架構或流程有重大變更時，同步更新 `doc/spec.md`。
- 完成 task-tracking 中的項目時，將對應的 `- [ ]` 改為 `- [x]`。
- API 變更需同步更新 spec 中的 3.5 端點總覽表。
