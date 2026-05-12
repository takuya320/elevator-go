# elevator-go

複数台のエレベーターをホール呼びとかご内行先で動かすシミュレータ。エレベーターの運行ロジックをドメインとして整理してみたかったのが出発点。実装は Go、構成は DDD + クリーンアーキテクチャ。

![demo](docs/demo.gif)

ブラウザ UI からホールボタンとかご内行先ボタンを押せる。複数タブを開いても SSE で状態が同期する。

## 動かす

```bash
cd web && pnpm install && pnpm run build && cd ..  # 初回のみ
go run .
```

`:8080` で立ち上がる。UI は `/`、Swagger UI は `/docs`。
auto-tick の goroutine が裏で回っていて、既定で 1 秒ごとに状態が進む。

主な環境変数:

| 変数                      | 既定       | 用途                      |
| ------------------------- | ---------- | ------------------------- |
| `ADDR`                    | `:8080`    | リッスンアドレス          |
| `TICK_INTERVAL_MS`        | `1000`     | auto-tick の間隔          |
| `FLOOR_MIN` / `FLOOR_MAX` | `1` / `10` | 階数範囲（地下は負値）    |
| `ELEVATOR_COUNT`          | `2`        | 号機台数                  |
| `LOG_FORMAT`              | `text`     | `json` で構造化ログ       |
| `LOG_DEBUG`               | unset      | セットすると DEBUG レベル |

例えば `FLOOR_MIN=-2 FLOOR_MAX=20 ELEVATOR_COUNT=4 go run .` で地下 2 階〜20 階の 4 号機構成になる。

## API

エンドポイントは OpenAPI 仕様（[docs/openapi.yaml](docs/openapi.yaml)）が一次ソース。Swagger UI でブラウズするのが手っ取り早い。MVP として動いているのは以下:

- `GET /` … React UI（embed 済み）
- `GET /events` … SSE。接続直後に現在状態、以降は tick ごとに差分
- `GET /floors/{floor}/elevators` … その階から見えるエレベーター
- `POST /floors/{floor}/hall-calls` … ホール呼び
- `POST /elevators/{elevatorId}/car-calls` … かご内行先
- `POST /simulation/tick` … 手動で 1 tick 進める
- `POST /simulation/reset` … リセット

admin 系（ドア操作、stop/resume、PATCH 系）は OpenAPI には書いてあるが未実装で、呼ぶと 501 が返る。

## 構成

```
interface/http  →  usecase  →  domain
                      ↑
             infrastructure
```

- `internal/domain/elevator` … Entity / VO / 集約（ElevatorBank）/ Domain Service。stdlib のみ。
- `internal/usecase` … Find → 処理 → Save の協調。Clock・IDGenerator・Locker などは Port として宣言してインフラに差し替えてもらう。
- `internal/infrastructure` … in-memory repository、mutex locker、system clock、UUID generator。
- `internal/interface/http` … OpenAPI から生成した型（`oapi/`）と chi ハンドラ（`server/`）。Broadcaster / SSE / auto-ticker / 静的配信もここ。
- `web/` … Vite + React + TypeScript。`pnpm run build` の出力が `server/webdist/` に置かれて、`embed.FS` でバイナリに同梱される。

`time.Now()` も ID 生成もドメインからは呼ばない。テストで決定論的に差し替えるための設計上の選択で、Clock / IDGenerator を Port にしている。配車は「距離 → idle 優先 → ID 昇順」と決め打ちにしていて、同じ状態からは必ず同じ号機が割り当たる。

設計の細かいところはドキュメントに分けてある:

- [docs/domain.md](docs/domain.md) … ドメインモデル
- [docs/behavior.md](docs/behavior.md) … 状態遷移と配車ルール、tick セマンティクス
- [docs/test-cases.md](docs/test-cases.md) … テストケース
- [docs/api.md](docs/api.md) … API 解説

## 開発

```bash
go build ./...
go vet ./...
go test ./...
```

OpenAPI を編集したら `go generate ./...` で `internal/interface/http/oapi/api.gen.go` を再生成する。新規エンドポイントは `server.Handler` が `oapi.Unimplemented` を埋め込んでいるので、対応メソッドをオーバーライドして UseCase を呼べばいい。ドメイン sentinel から HTTP コードへの変換は `internal/interface/http/server/errors.go` の `errorMapping` 一箇所に集約してあるので、ハンドラ側に switch を散らさない。

フロントは `web/` 配下:

```bash
cd web
pnpm install     # 初回のみ
pnpm run dev     # Vite dev server (:5173)、API は :8080 に proxy
pnpm run build   # webdist/ に出力
```

ビルドし直して `go run .` を再起動すれば新しい UI が embed される。

## ライセンス

[MIT License](LICENSE)
