# mdimg

提取 Markdown 文檔引用的圖片路徑。以 Go 實作，編譯後為單一執行檔，不需要安裝 Go、Python 或其他執行環境。

## 使用

```sh
mdimg README.md           # 全部圖片
mdimg -l README.md        # 只輸出本地圖片
mdimg -w README.md        # 只輸出網路圖片
mdimg README.md -l        # 參數也可放在檔名之後
mdimg --local README.md
mdimg --web README.md
mdimg --help
mdimg -- -example.md      # 檔名以 - 開頭
mdimg -l README.md > images.txt
```

Windows PowerShell 使用 `.\mdimg.exe -l README.md`。檔案路徑含空格時請加引號。

每行輸出一個路徑，依圖片在文檔中的出現順序排列，保留重複引用。沒有圖片時不輸出內容，成功退出。錯誤訊息寫入 stderr；退出碼為 0（成功）、1（讀寫失敗）、2（參數錯誤）。`-l` 和 `-w` 互斥。

## 支援的語法與路徑規則

- CommonMark 行內圖片：`![描述](images/photo.png "標題")`。
- 完整、折疊及簡寫參照式圖片：`![描述][id]`、`![id][]`、`![id]`，解析對應的 `[id]: path` 定義。
- HTML `<img src="...">`，支援單引號、雙引號、無引號及跨行標籤。
- 忽略程式碼區塊、行內程式碼、HTML 註解、一般超連結及未定義的參照。
- 空圖片目的地不輸出。只讀取 `<img>` 的 `src`，不解析 `srcset`、CSS、MDX 或 Wiki 圖片語法。
- `-w` 包含 `http://`、`https://`、`//` 開頭的網址（協定名稱不分大小寫）。
- `-l` 包含相對路徑、Unix 絕對路徑、Windows 磁碟／反斜線 UNC 路徑，以及 `file:` URL。`//` 一律視為網路網址。
- `data:`、`ftp:` 等其他 URI 協定只在不加篩選時輸出。
- 保留來源中的路徑形式，解碼 Markdown 跳脫及 HTML 字元實體，不解碼 `%20` 等 URL 百分比編碼。不轉換成絕對路徑，也不檢查圖片是否存在或下載圖片。相對路徑應由使用者按 Markdown 檔案所在目錄解讀。

## 編譯

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
