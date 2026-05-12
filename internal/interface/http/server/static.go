package server

import (
	"embed"
	"io/fs"
	"net/http"
)

// webdist/ は web/ の Vite ビルド出力先（embed の制約で同パッケージ配下に置く）。
// build 出力は git 管理対象外で、フレッシュ clone 直後は .gitkeep のみ。
//
//go:embed all:webdist
var webDistFS embed.FS

const notBuiltHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <title>elevator-go</title>
  <style>body{font-family:system-ui,sans-serif;max-width:600px;margin:4rem auto;line-height:1.6;padding:0 1rem;}</style>
</head>
<body>
  <h1>UI 未ビルド</h1>
  <p><code>cd web &amp;&amp; pnpm install &amp;&amp; pnpm run build</code> を実行してから再起動してください。</p>
  <p>API は <a href="/docs">/docs</a> から確認できます。</p>
</body>
</html>
`

// 未ビルド (index.html がない) なら案内をハードコードで返す。
// ある場合は通常の FileServer で配信する。
func StaticHandler() (http.Handler, error) {
	sub, err := fs.Sub(webDistFS, "webdist")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(notBuiltHTML))
		}), nil
	}
	return http.FileServer(http.FS(sub)), nil
}
