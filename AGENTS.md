# AGENTS.md

プロジェクト全体の設計・規約・主要コマンドは `CLAUDE.md` と `docs/` が一次ソース。ここでは重複を避け、Cursor Cloud 環境特有の注意点のみ記す。

## Cursor Cloud specific instructions

対象サービスは 1 つの product（エレベーターシミュレータ）で、backend（Go, `:8080`）と frontend（Vite + React, `web/`）から成る。標準コマンドは `CLAUDE.md`「主要コマンド」/ `README.md`/ `Makefile` を参照。

### ツールチェーンの前提（スナップショットに含まれる）
- **Go 1.25** が必須（`go.mod` の `go 1.25`）。ベースイメージの `go` は 1.22 なので、Go 1.25 を `/usr/local/bin/go` に配置してある。`GOTOOLCHAIN=auto` のままだと `go` がネットワーク越しに 1.25 toolchain を落とそうとして失敗する環境なので、ローカルに 1.25 本体があることが前提（`go version` が `go1.25.x` を返せば OK）。
- **golangci-lint v2**（`/usr/local/bin/golangci-lint`）を導入済み。設定は `.golangci.yml`（v2 形式）。
- **Node 22 / pnpm**（`packageManager: pnpm@10.33.2`、corepack 経由）。

### 起動・確認の非自明な点
- backend は API と UI を **同一 :8080** で配信する。`go run .` の前に **`cd web && pnpm run build`** を一度実行しておかないと、embed される UI は placeholder（`internal/interface/http/server/webdist/index.html`）のままで実際の React 画面が出ない。build 出力先 `internal/interface/http/server/webdist/` は placeholder 以外 gitignore 対象。
- auto-ticker が既定 1 秒ごとに状態を進めるため、`POST /simulation/tick` を叩かなくても状態は勝手に進む。手動 tick と auto-tick が混ざることに注意。
- 別オリジンで動かす dev 構成（Vite `:5173` → backend `:8080` proxy、`cd web && pnpm run dev`）では backend 側に `CORS_ALLOWED_ORIGINS=http://localhost:5173` が必要（compose もこれを設定している）。単一オリジン（embed 配信）なら CORS 設定は不要。
- `pnpm install` 時に `Ignored build scripts: esbuild` の警告が出るが、Vite build は問題なく通る（対応不要）。
