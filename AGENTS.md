# 專案協作指南

本文件適用於整個儲存庫。文件與協作說明使用繁體中文；程式識別字、CLI 說明與錯誤訊息沿用現有英文風格。

## 強制約束

- 如果新的需求與當前 `AGENTS.MD` 發生衝突必須向用戶確認.
- 在文件變更之後主動輸出變更對比內容.
- 代碼需要符合 google-style.

## 專案概覽

`mdimg` 是以 Go 實作的 Markdown 圖片路徑提取與檢查工具，編譯後為單一執行檔。模組名稱為 `github.com/alctny/mdimg`，最低 Go 版本為 1.23.0。

- `main.go`：CLI 參數解析、檔案探索、圖片解析、路徑轉換、存在性檢查與輸出。
- `main_test.go`：解析、CLI、版本、檔案探索、本地路徑及 HTTP 檢查測試。
- `testdata/example.md`：手動驗證用 Markdown 範例；其中的圖片不保證實際存在。
- `README.md`：使用方式、路徑規則與建置說明；部分工作區可能顯示為 `README.MD`，修改現有檔案即可，不另建大小寫不同的副本。
- `build.sh`：共用建置邏輯；`build.bat` 僅轉交參數並保留退出碼。
- `go.mod`、`go.sum`：依賴定義；使用 Goldmark 解析 Markdown、`golang.org/x/net/html` 解析 HTML。
- `dist/`：被 Git 忽略的建置輸出與 `SHA256SUMS`。

## 修改方式

- 開始前檢查 `git status` 與相關差異，保留使用者既有的工作區及暫存區修改。
- 維持目前單一 `main` 套件的簡潔結構，依任務需要修改，避免順帶重構或新增無關依賴。
- Go 程式碼使用 `gofmt`。路徑處理沿用 `filepath` 與現有輔助函式，兼顧 Windows、macOS、Linux。
- CLI 行為變更須同步更新 `usage`、README 與對應測試。
- 保留 `run(args, out, errOut) int` 的可測試入口；`os.Exit` 留在 `main`。

## 必須保留的行為

除非任務明確要求變更，下列行為屬於既有介面：

- 透過 Goldmark AST 與 HTML tokenizer 提取圖片，不以正規表示式取代解析器。支援行內／參照式圖片及 HTML `img src`，忽略程式碼、HTML 註解、一般連結、未定義參照及空地址。
- 圖片依文檔出現順序輸出，保留重複引用。預設按文檔分組，檔名不加冒號，圖片縮排四個空格，保留既有 ANSI 顏色與檢查標記。
- `-i` 僅隱藏檔名與縮排；`--convert` 僅影響顯示，不改寫文檔或改變檢查使用的地址。百分比編碼只解碼一次，`+` 保持原樣，無效編碼的顯示保留原文。
- 未指定檔案時探索當前目錄的 `.md` 檔案，副檔名不分大小寫；`-r` 才遞迴。明確指定檔案時不額外探索。保留位置參數、可重複的 `-f` 與 `--` 結束選項語法。
- `-l`／`--local` 與 `--web` 為互斥篩選；`-w` 是網路檢查開關。只有同時啟用 `-c` 與 `-w` 才發出 HTTP 請求。
- 相對圖片路徑預設以 Markdown 所在目錄為基準。`-p` 相對於執行目錄，`-P` 相對於各文檔目錄，兩者互斥；以 `./` 或 `.\` 開頭的圖片路徑忽略前綴，絕對路徑不加前綴。
- 本地檢查接受兩種路徑分隔符與 `file:` URL，目錄不算圖片檔案。`http://`、`https://`、`//` 分類為網路地址；其他 URI 協定可列出，但不屬於本地或網路篩選。
- HTTP 檢查使用 GET、跟隨重導向、以最終 2xx 判定成功，每個請求逾時為 10 秒；`//` 使用 HTTPS。關閉回應本文，不下載完整圖片。
- 退出碼：`0` 表示掃描完成（包含發現缺失圖片），`1` 表示文檔讀取、目錄掃描或輸出失敗，`2` 表示參數錯誤。錯誤寫入 stderr，單一文檔讀取失敗後仍繼續處理其他文檔。

## 驗證

在專案根目錄執行。首次需要依賴時使用 `go mod download`。

修改 Go 程式碼後，格式化改動檔案並執行：

```sh
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w" -o mdimg .
```

Windows 建置輸出改用 `-o mdimg.exe`。可用 `go run . -f testdata/example.md` 與 `go run . --help` 驗證相關 CLI 行為。

- 功能變更或修正錯誤時，沿用現有表格驅動測試，補上能重現問題的案例；檢查退出碼、stdout 與 stderr，輸出測試保留 ANSI 控制碼。
- 檔案測試使用 `t.TempDir()`；HTTP 測試使用 `httptest.NewServer`，不依賴公開網站。
- 會呼叫 `os.Chdir` 的測試影響整個程序，不加入 `t.Parallel()`。
- 僅修改文件時，檢查內容與程式／腳本是否一致即可，無需為此重新建置或增加測試。
- 修改 `build.sh` 時先執行 `sh -n build.sh`，再依變更驗證相關選項及目標。交叉編譯成功不等同於目標平台實機測試。
- 完成時說明修改內容、實際執行的驗證與未完成的檢查，不宣稱未執行的測試已通過。

## 建置與版本

- `build.sh` 保持 POSIX `sh` 相容，並能從任意工作目錄呼叫；Windows 透過 Git for Windows 的 `sh` 執行相同腳本。
- `sh build.sh` 建置本機平台；`sh build.sh -a` 建置 Darwin、Linux、Windows 的 amd64／arm64 六種組合。預設平台取自 `GOHOSTOS`／`GOHOSTARCH`。
- 建置使用 `CGO_ENABLED=0`，維持無需 C 編譯器的發行流程。
- 版本由最近可達的 Git tag 取得，日期使用 `Asia/Shanghai`，另注入完整 commit hash 與 tag；透過 linker `-X main.<欄位>` 設定。建置腳本不建立 commit 或 tag。
- UPX 僅在指定 `-x` 時使用；壓縮或驗證失敗時保留未壓縮版本。不強制壓縮 macOS 執行檔。
- `dist/SHA256SUMS` 僅記錄本次建置產物。不要提交 `dist/` 或根目錄的 `mdimg`／`mdimg.exe`。
