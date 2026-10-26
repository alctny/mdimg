# CLI 圖片範例與驗證對照

本文件同時是可掃描的圖片樣本與手動驗證指南，涵蓋目前 CLI 的全部參數（含 `--wikilinks`）。命令預設在儲存庫根目錄執行，`mdimg` 代表目前原始碼建置的執行檔；macOS／Linux 可換成 `./mdimg`，PowerShell 可換成 `.\mdimg.exe`。也可用 `go run .` 代替，但要驗證退出碼時請用執行檔：`go run` 不會原樣傳回程式的非零退出碼。

`images/present.svg` 是隨附的存在樣本，其餘本地圖片刻意不提供。不要建立同名缺失檔案，否則檢查結果會改變。網路地址只示範分類，不假設公開網站可存取。下方命令與預期輸出放在程式碼內，不會成為額外圖片；忽略案例另列於第 5 節。

## 1. Markdown 與 HTML 圖片

![存在的本地圖片](images/present.svg)
![缺失的本地圖片](images/missing.png)
![含空格](<images/my photo.png>)
![完整參照][logo]
![logo][]
![logo]
![重複引用](images/present.svg)

[logo]: images/present.svg "參照圖片"

內嵌 HTML：<img src="images/present.svg" alt="存在的 HTML 圖片">

<div>
  <img src="images/html.png?a=1&amp;b=2" alt="HTML 實體">
</div>

## 2. 本地路徑、解碼與前綴

![明確相對路徑](./images/present.svg)
![反斜線相對路徑](.\images\present.svg)
![反斜線分隔符](images\present.svg)
![父目錄](../testdata/images/present.svg)
![file URL](file:images/present.svg)
![目錄不是圖片檔案](images/)
![百分比編碼空格](images/my%20photo.png)
![UTF-8、加號與單次解碼](images/%E5%9C%96+a%2520.png)
![無效百分比編碼](images/bad%zz.png)
![Markdown 跳脫](images/a\(1\).png)
![跳脫空格](images/a\ b.png)
![Markdown 實體](images/a&amp;b.png)
![Unix 絕對路徑](/mdimg-example-missing/photo.png)
![Windows 絕對路徑](C:\mdimg-example-missing\photo.png)

兩個絕對路徑是平台示例，不假設它們存在或可跨平台存取。`file:` 歸類為本地；檢查只判斷是否為可存取的非目錄檔案，不驗證圖片內容。

## 3. 網路地址與其他 URI

![HTTPS](https://example.com/photo.png)
![HTTP](http://example.com/photo.png)
![省略協定](//cdn.example.com/banner.webp)
![URL 編碼](https://example.com/my%20photo.png?q=a+b)
![其他 URI](data:image/png;base64,AAAA)

`http://`、`https://`、`//` 屬於網路地址，`//` 檢查時使用 HTTPS。`data:` 可列出，但不屬於 `--local` 或 `--web`，也沒有專用存取檢查。

## 4. Obsidian 圖片嵌入

以下五筆僅在啟用 `--wikilinks` 時加入輸出，顯示尺寸不屬於地址，重複引用仍保留。

![[images/present.svg]]
![[images/wiki photo.PNG|300]]
![[images/present.svg|300x200]]
![[./images/present.svg|120]]
![[https://example.com/wiki.webp?size=2#preview|300]]

## 5. 不應提取的內容

一般連結：[圖片連結](ignored-link.png)。未定義參照：![未定義][unknown-image]。空地址：![空地址]()。

跳脫圖片：\![跳脫](ignored-escaped.png)。一般 wikilink：[[ignored-link.png]]。

非圖片嵌入：![[note.md]] ![[audio.mp3]] ![[video.mp4]] ![[document.pdf]]。

行內程式碼：`![忽略](ignored-inline.png)`、`![[ignored-inline-wiki.png]]`。

<!-- ![忽略註解](ignored-comment.png) <img src="ignored-comment-html.png"> ![[ignored-comment-wiki.png]] -->

```md
![忽略圍欄程式碼](ignored-fenced.png)
<img src="ignored-fenced-html.png">
![[ignored-fenced-wiki.png]]
```

    ![忽略縮排程式碼](ignored-indented.png)
    ![[ignored-indented-wiki.png]]

只提供 srcset，不提取：<img srcset="ignored-srcset.png 1x, ignored-srcset-2x.png 2x" alt="無 src">

## 6. 基準輸出

```sh
mdimg -t -i -f testdata/example.md
```

預期 stdout 恰有以下 28 行，stderr 為空、退出碼為 `0`。未加 `--convert` 時，下列反斜線在所有平台均保留原樣。

```text
images/present.svg
images/missing.png
images/my photo.png
images/present.svg
images/present.svg
images/present.svg
images/present.svg
images/present.svg
images/html.png?a=1&b=2
./images/present.svg
.\images\present.svg
images\present.svg
../testdata/images/present.svg
file:images/present.svg
images/
images/my%20photo.png
images/%E5%9C%96+a%2520.png
images/bad%zz.png
images/a(1).png
images/a b.png
images/a&b.png
/mdimg-example-missing/photo.png
C:\mdimg-example-missing\photo.png
https://example.com/photo.png
http://example.com/photo.png
//cdn.example.com/banner.webp
https://example.com/my%20photo.png?q=a+b
data:image/png;base64,AAAA
```

加上 `--wikilinks` 後，依序在末尾增加五行，共 33 行：

```text
images/present.svg
images/wiki photo.PNG
images/present.svg
./images/present.svg
https://example.com/wiki.webp?size=2#preview
```

## 7. 全部參數與組合的預期結果

下表「參數」一律接在 `mdimg -f testdata/example.md` 後面。未特別註明時，stderr 為空、退出碼為 `0`；圖片缺失不會使退出碼變成非零。

| 參數 | 預期結果 |
| --- | --- |
| 無 | 首行 `testdata/example.md`，其後 28 筆圖片各縮排四格；本地及其他 URI 為黃色，網路為藍色，均有 ANSI 控制碼 |
| `-i` | 隱藏檔名與縮排，保留顏色與順序 |
| `-t` | 29 行純文字（檔名及 28 筆地址），無 ANSI、縮排或檢查標記 |
| `-t -i` | 第 6 節的 28 行輸出 |
| `--convert -t -i` | 百分比編碼解碼一次，本地路徑改用本機分隔符，URL 保留 URL 分隔符；不改寫本文件 |
| `--wikilinks -t -i` | 第 6 節的 33 行輸出；可與篩選、檢查、前綴合用 |
| `-l` 或 `--local` | 只列出前 23 筆本地地址，包含 `file:`，排除網路與 `data:` |
| `--web` | 只列出四筆網路地址；不發送 HTTP 請求 |
| `-c` | 本地存在為綠色 `[v]`，缺失與目錄為紅色 `[x]`，四筆網路為藍色 `[w]`；`data:` 為 `[x]` |
| `-c -i` | 保留上述標記與顏色，隱藏檔名及縮排 |
| `-c -n` | 只保留 `[x]`，不輸出存在圖片與未檢查網路圖片 |
| `-c -t` | 仍執行檢查，但輸出與 `-t` 相同，無檢查標記 |
| `-c -n -t -i` | 只輸出失敗項目的原始地址，無其他資訊 |
| `-c -F` | 只輸出一行 `testdata/example.md`，無顏色或標記 |
| `-c -F -n -i -t` | 仍只輸出一行檔名；`-F` 優先於 `-i`，本文件有失敗項目 |
| `-c --web -n` 或 `-c --web -F -n` | stdout 為空；未檢查的網路項目不算失敗 |
| `-w` | 與未加參數相同，不發送 HTTP 請求 |
| `-c -w --web` | 發送 GET；最終 2xx 為綠色 `[wv]`，其他狀態、連線失敗或逾時為紅色 `[wx]`；結果取決於網路 |
| `-c -w --web -n` | 只保留 `[wx]`；加 `-F` 則只在有失敗時輸出一次檔名 |
| `-c -p testdata` | 一般相對路徑以執行目錄的 `testdata/` 為基準，結果與預設相同 |
| `-c -P .` | 一般相對路徑以本文件目錄為基準，結果與預設相同 |
| `-c -p .` 或 `-c -P ..` | 一般 `images/present.svg` 改以儲存庫根目錄為基準，預設為 `[x]`；`./images/present.svg` 與 `.\images\present.svg` 仍為 `[v]` |
| `-c --convert` | 檢查結果與 `-c` 相同，僅顯示地址轉換 |
| `-F -n -p . -w` | 未啟用 `-c`，這些參數不影響列出結果；`-P` 未搭配 `-c` 也相同 |
| `-r` | 已由 `-f` 指定文件，不額外探索其他文件 |

`--convert` 的代表結果（以下以 macOS／Linux 分隔符顯示）：

| 原始地址 | 轉換後 |
| --- | --- |
| `images/my%20photo.png` | `images/my photo.png` |
| `images/%E5%9C%96+a%2520.png` | `images/圖+a%20.png`，`+` 不變、`%2520` 不會解碼成空格 |
| `images/bad%zz.png` | 原文保留；本地檢查失敗 |
| `images\present.svg` | `images/present.svg` |
| `https://example.com/my%20photo.png?q=a+b` | `https://example.com/my photo.png?q=a+b` |
| `file:images/present.svg` | 原文保留，不轉換 URL 分隔符 |

## 8. 檔案選擇、探索、說明與版本

下列都是完整命令。選項可位於檔名前後；同一文件重複指定時，會重複處理與輸出。

| 命令 | 預期結果 |
| --- | --- |
| `mdimg testdata/example.md` | 位置參數與 `-f testdata/example.md` 相同 |
| `mdimg -t -i -f testdata/example.md -f testdata/example.md` | 基準 28 行完整重複兩次，共 56 行 |
| `mdimg -c -F testdata/example.md -f testdata/example.md` | 檔名輸出兩次 |
| `mdimg -- testdata/example.md` | `--` 後全部當作檔名；結果同預設 |
| `mdimg -- -example.md` | 讀取名為 `-example.md` 的檔案；需先準備此檔案，否則退出 `1` |
| `mdimg -f ./-example.md` | 同樣可讀取以 `-` 開頭的檔名 |
| `mdimg` | 只探索執行目錄的 `.md`／`.MD` 文件，不會進入 `testdata/` |
| `mdimg -r` | 遞迴探索，包含本文件；實際輸出取決於工作目錄中的文件 |
| `mdimg -h` 或 `mdimg --help` | stdout 為使用說明，退出 `0` |
| `mdimg -v` | 只輸出版本號；未注入版本時為 `dev`，退出 `0` |
| `mdimg -V` 或 `mdimg --version` | 顯示版本、日期、commit、tag、開發者與描述，退出 `0` |

## 9. 錯誤與退出碼

| 命令／情境 | 預期結果 |
| --- | --- |
| `mdimg -c -n -f testdata/example.md` | 即使有缺失圖片，退出仍為 `0` |
| `mdimg -f testdata/does-not-exist.md -f testdata/example.md` | stderr 報讀取失敗，繼續輸出本文件，最後退出 `1` |
| `mdimg -f testdata` | 讀取目錄失敗，stderr 報錯，退出 `1` |
| `mdimg -f`、`mdimg -p`、`mdimg -P` | stdout 為空；stderr 為 `mdimg: <選項> requires an argument`，退出 `2` |
| `mdimg -f -c` | `-f` 缺少有效參數，退出 `2` |
| `mdimg -l --web` | stdout 為空；stderr 含 `mdimg: specify at most one filter` 與使用說明，退出 `2` |
| `mdimg -p . -P .` | stdout 為空；stderr 為 `mdimg: specify only one of -p and -P`，退出 `2` |
| `mdimg -p . -p .` 或 `mdimg -P . -P .` | 重複前綴也會報上述錯誤，退出 `2` |
| `mdimg --bad` | stdout 為空；stderr 含 `mdimg: unknown option "--bad"` 與使用說明，退出 `2` |
| 探索時遇到不可讀目錄 | stderr 報錯，退出 `1`；需要另外準備權限情境 |
| 圖片列表寫入 stdout 失敗 | stderr 含 `mdimg: write output:`，退出 `1`；需要失敗的輸出目的地 |

## 10. 單一文件無法固定的結果

本文件可覆蓋參數用法與圖片提取樣本，但不能單獨保證所有執行結果。HTTP 成敗、重導向、逾時、空文件、多文件、大小寫副檔名探索、讀寫權限與平台絕對路徑，還需要伺服器、其他檔案或特定環境。

以下既有測試使用暫存檔與本機 `httptest.NewServer`，可補充驗證探索、路徑、HTTP 200／404、重導向、未啟用檢查時不送請求，以及輸出組合，不依賴公開網站：

```sh
go test ./... -run '^(TestCLI|TestPlainText|TestFilenamesOnly|TestVersion|TestChecksAndDiscovery|TestExplicitDocumentRelativePaths|TestWebChecks|TestLocalPath|TestConvert|TestImagePaths|TestWikilinkImagePaths|TestWikilinksCLI|TestWikilinksCheckParity)$'
```

此命令不代表已覆蓋 HTTP 10 秒逾時、目錄掃描權限或 stdout 寫入失敗等所有環境錯誤；這些結果需另行驗證。
