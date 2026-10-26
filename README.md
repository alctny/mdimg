# mdimg

提取 Markdown 文檔引用的圖片路徑。以 Go 實作，編譯後為單一執行檔，不需要安裝 Go、Python 或其他執行環境。

## 安裝

已安裝 Go 1.23 或更新版本時，可直接從 GitHub 下載並安裝，無需先 clone 儲存庫：

```sh
go install github.com/alctny/mdimg@latest
```

更新到最新版本時，重新執行相同命令即可。`@latest` 優先選擇最新穩定版本標籤；沒有版本標籤時，使用預設分支的最新提交。若要安裝 `main` 分支的最新程式碼：

```sh
go install github.com/alctny/mdimg@main
```

執行檔安裝到 `go env GOBIN` 指定的目錄；若未設定，則使用 `go env GOPATH` 下的 `bin` 目錄（預設為 macOS／Linux 的 `$HOME/go/bin` 或 Windows 的 `%USERPROFILE%\go\bin`）。請將該目錄加入 `PATH`，之後即可在任意目錄執行 `mdimg --help`；Windows 的執行檔名稱為 `mdimg.exe`。

## 使用

```sh
mdimg                         # 列出當前目錄全部 .md 文檔的圖片地址
mdimg -f abc.md                # 只列出指定文檔的圖片地址
mdimg -i -f abc.md             # 只輸出圖片地址，隱藏文檔名及縮排
mdimg --convert -f abc.md      # 解碼圖片地址並使用本機路徑分隔符
mdimg -r                      # 遞迴掃描子目錄（副檔名不分大小寫）
mdimg -c -f abc.md             # 檢查本地圖片，網路圖片標記為未檢查
mdimg -c -r -n                # 遞迴檢查，只顯示缺失圖片及其文檔
mdimg -c -w -f abc.md          # 同時檢查 HTTP(S) 圖片是否可訪問
mdimg -c -f path/abc.md -p assets  # 圖片相對於執行目錄的 assets/
mdimg -c -f path/abc.md -P assets  # 圖片相對於 path/assets/
mdimg --help
mdimg -v                      # 僅輸出版本號 v1.0.1
mdimg -V                      # 完整版本資訊（等同 --version）
```

Windows PowerShell 使用 `.\mdimg.exe`。路徑含空格時請加引號。`-f`、`-p`、`-P` 必須提供參數；`-p` 與 `-P` 不可同時指定。可重複使用 `-f` 指定多個文檔；明確指定檔案時，`-r` 不額外掃描目錄。

預設按文檔分組輸出，文檔名頂格且不加冒號，圖片地址換行後縮排四個空格。本地圖片地址顯示黃色，HTTP(S) 網路地址顯示藍色；其他 URI 使用黃色。依文檔中的出現順序排列並保留重複引用。未指定 `-c` 時，`-p`、`-P`、`-n`、`-w` 不影響此模式。保留舊版位置參數（`mdimg abc.md`）、`-l`／`--local` 本地篩選及 `--web` 網路篩選；`-w` 現在表示檢查網路圖片，不再表示篩選。兩種篩選互斥。以 `-` 開頭的檔名可使用 `mdimg -- -example.md` 或 `-f ./-example.md`。

`-i` 適用於所有模式：只隱藏文檔名和縮排，圖片地址仍逐行輸出並保留顏色；搭配 `-c` 時保留檢查標記，搭配 `-n` 時仍只輸出失敗項目。

`--convert` 適用於所有輸出模式：在既有 Markdown 跳脫及 HTML 實體解析後，將 `%20`、`%23`、UTF-8 百分比編碼等還原為原始字元，並將本地檔案路徑的 `/`、`\` 轉為目前系統的分隔符（Windows 為 `\`，Linux/macOS 為 `/`）。只解碼一次，`+` 不轉成空格；含無效百分比編碼的地址保留其編碼原文。URL（包含 `file:`）保留 URL 分隔符。此參數只調整顯示，不修改 Markdown 檔案，也不改變檢查所用的地址或 prefix 規則。

檢查模式按文檔分組，檔名頂格且不加冒號，圖片縮排四個空格：

```text
abc.md
    [x] missing.png
    [v] images/photo.jpg
    [w] https://example.com/unchecked.png
```

- `[v]` 綠色：本地檔案存在；目錄不視為圖片檔案。
- `[x]` 紅色：本地檔案不存在或無法存取。
- `[w]` 藍色：HTTP(S) 圖片未檢查。
- 加上 `-w` 後，網路圖片以 `[wv]` 綠色表示可訪問，`[wx]` 紅色表示檢查失敗。
- `-n` 只輸出 `[x]` 和 `[wx]`，隱藏其他圖片及沒有失敗項目的文檔。沒有圖片的文檔不輸出。

顏色使用 ANSI 控制碼。網路檢查使用 GET、跟隨重導向，以最終 HTTP 2xx 判定成功；每個請求最多等待 10 秒，不讀取完整圖片。`//` 開頭的網路地址使用 HTTPS。連線失敗或逾時也會標為 `[wx]`。

錯誤訊息寫入 stderr；退出碼為 0（掃描完成，包含發現缺失圖片）、1（文檔讀取、目錄掃描或輸出失敗）、2（參數錯誤）。某個文檔讀取失敗時仍繼續處理其他指定文檔。

## 支援的語法與路徑規則

- 支援 CommonMark 行內圖片、完整／折疊／簡寫參照式圖片，以及 HTML `<img src="...">`。
- 忽略程式碼區塊、行內程式碼、HTML 註解、一般超連結及未定義的參照。空圖片地址不輸出，不解析 `srcset`、CSS、MDX 或 Wiki 圖片語法。
- 解析 Markdown 標點跳脫、反斜線跳脫空格及 HTML 字元實體。輸出保留 `%20` 等百分比編碼；本地檢查時再解碼（包含空格、中文及特殊字元），`+` 保持為加號。
- 本地檢查接受 `/` 和 `\` 路徑分隔符及 `file:` URL。絕對路徑直接檢查，不加前綴；跨系統磁碟或共享路徑仍需在執行系統上可存取。
- 相對圖片路徑預設以其 Markdown 文檔所在目錄為基準。以 `./` 或 `.\` 開頭的圖片路徑永遠以文檔所在目錄為基準，忽略 `-p` 和 `-P`。其他相對路徑（例如 `path/to/img`）使用指定前綴：`-p` 相對於執行目錄，`-P` 相對於各 Markdown 文檔目錄；前綴本身也可為絕對路徑。
- `http://`、`https://`、`//` 為網路圖片；`file:` 為本地圖片。其他 URI 協定（例如 `data:`、`ftp:`）仍可列出，但不提供其專用存取檢查，檢查時通常標為 `[x]`。

## 編譯

目前版本為 `v1.0.1`，開發者為 **GPT6-Astra 和 Ant**。`-V`／`--version` 顯示版本、編譯日期、Git commit、Git tag、開發者及簡要描述。

建置腳本預設使用 Asia/Shanghai 當日日期，自動讀取 `git rev-parse HEAD` 與 HEAD 上的 tag。HEAD 沒有 tag 時顯示 `untagged`，無法取得 commit 時顯示 `unknown`；不會自動建立 tag 或 commit。版本資訊使用 Go linker 的 `-X main.<欄位>=<值>` 注入。直接執行 `go build` 而未注入時，日期預設為 `2026-10-26`，commit/tag 為 `unknown`。

可透過腳本參數覆寫資訊；`build.sh` 和 `build.bat` 均轉交這些參數：

```sh
go run ./scripts/build.go -date 2026-10-26
go run ./scripts/build.go -target linux/amd64 -date 2026-10-26 -skip-upx
go run ./scripts/build.go -version v1.0.1 -date 2026-10-26 -commit abc123 -tag v1.0.1
```

另支援 `-developers`、`-description` 覆寫文字（含空格時加引號）。不指定 `-target` 時建置全部六個平台；指定時只建置該平台，校驗檔寫入 `dist/SHA256SUMS-linux-amd64` 等對應名稱。預設的 `dist/SHA256SUMS` 仍用於完整建置。

需要 Go 1.23 或更新版本。首次編譯需要下載 `go.mod` 中固定版本的解析套件。

```sh
go mod download
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w" -o mdimg .
```

Windows 可使用 `go build -trimpath -ldflags="-s -w" -o mdimg.exe .`。

使用建置腳本，一次產生所有平台的獨立執行檔及 SHA-256 校驗檔（腳本可從任意工作目錄呼叫）：

```sh
# macOS / Linux
sh build.sh
```

```bat
rem Windows（PowerShell 中使用 .\build.bat）
build.bat
```

兩個腳本共用 Go 建置程式，也可在專案根目錄直接執行：

```sh
go run ./scripts/build.go
```

建置時使用 `-trimpath -buildvcs=false -ldflags="-s -w -buildid="`，移除符號表、DWARF 除錯資訊、Go build ID、版本控制資訊及本機來源路徑。執行時必需的載入／連結資料與 Go runtime 中繼資料仍會保留，不能刪除所有此類資料而同時確保程式正常執行。

腳本自動在 PATH 中尋找 `upx`。有安裝時預設使用 `--best --ultra-brute`，啟用最高壓縮設定並嘗試更多壓縮組合（耗時較長），再以 `upx -t` 驗證，最後計算 SHA-256。沒有 UPX 時正常輸出未壓縮版本。若 UPX 不支援某個目標格式、無法啟動或壓縮／驗證失敗，會顯示警告並保留該目標的未壓縮執行檔；其他目標繼續建置。

本機 UPX 5.2.1 預設拒絕壓縮 macOS 執行檔；腳本保留這些目標的未壓縮版本，不使用 `--force-macos` 強制壓縮。

| 輸出檔案 | 平台 |
| --- | --- |
| `dist/mdimg-darwin-amd64` | macOS Intel |
| `dist/mdimg-darwin-arm64` | macOS Apple Silicon |
| `dist/mdimg-linux-amd64` | Linux x64 |
| `dist/mdimg-linux-arm64` | Linux ARM64 |
| `dist/mdimg-windows-amd64.exe` | Windows x64 |
| `dist/mdimg-windows-arm64.exe` | Windows ARM64 |
| `dist/SHA256SUMS` | 所有執行檔的 SHA-256 |

建置腳本設定 `CGO_ENABLED=0`，無需 C 編譯器或第三方動態函式庫。選擇對應系統與架構的檔案；macOS／Linux 複製後執行 `chmod +x mdimg-*`，即可用 `./mdimg-linux-amd64 example.md` 等方式執行，也可重新命名為 `mdimg` 並放進 PATH。macOS 版本未經 Apple 開發者簽署或公證。

跨平台編譯不等同於所有平台的實機測試；目前已在 Windows x64 執行測試及命令行驗證，其餘目標完成交叉編譯。
