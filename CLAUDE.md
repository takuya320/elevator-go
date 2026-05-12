# Claude 向けプロジェクトメモ

将来のセッションで Claude が即座に文脈を取り戻すための要点。
詳細は `docs/` 配下の各ドキュメントが一次ソース。

## このプロジェクト

Go 製のエレベーター運行シミュレータ。OpenAPI 定義 → chi で配信、DDD + クリーンアーキテクチャでドメイン層を組んだもの。MVP として 5 エンドポイント（フロアからのエレベーター取得・ホール呼び・かご内行先・tick・reset）が実装済み。残り（admin 系）は `oapi.Unimplemented` 経由で 501。

## 厳守ルール

1. **コメントは WHY のみ・日本語**。動作の説明は書かない（コードを読めば分かる）。
   非自明な制約・並行性契約・不変条件・防御的分岐の理由など、意思を補うものだけ書く。
2. **ドメイン (`internal/domain/elevator`) は stdlib のみ依存**。third-party を入れない。
3. **`time.Now()` をドメインから呼ばない**。`usecase.Clock` 経由で UseCase が注入する。
4. **ID 生成をドメインで行わない**。コンストラクタで受け取る。
5. **OpenAPI を変更したら `go generate ./...` を必ず走らせる**。
6. **エラー → HTTP マッピングは `internal/interface/http/server/errors.go` の `errorMapping` だけ**。
   ハンドラ側に switch を散らさない。
7. **完了報告の前に**: `go build ./... && go vet ./... && go test ./...` を走らせる。
   HTTP の挙動を変えたら `go run .` で起動 → curl まで確認する。「型が通った = 動く」ではない。

## レイヤと依存方向

```
interface/http  →  usecase  →  domain
                       ↑
              infrastructure
```

逆流禁止: domain は usecase を import しない、infrastructure は interface を import しない。

## ファイルレイアウト

```
elevator-go/
├── main.go                              依存組み立て + auto-ticker 起動
├── gen.go                               go:generate トリガ (build tag: generate)
├── oapi-codegen.yaml                    生成設定
├── docs/                                仕様書（一次ソース）
├── web/                                 Vite + React + TypeScript ソース
│                                        (pnpm run build → server/webdist/)
├── internal/
│   ├── domain/elevator/                 Entity / VO / 集約 / Domain Service
│   ├── usecase/                         Port + 5 UseCase + GetState
│   ├── infrastructure/
│   │   ├── clock/, id/, sync/           各 Port の実装
│   │   └── persistence/memory/          in-memory repo + simulation clock
│   └── interface/http/
│       ├── oapi/                        生成コード (api.gen.go)
│       └── server/                      handler / router / errors / convert / swagger
│           ├── broadcaster.go           SSE 用 pub/sub（バッファ満杯はドロップ）
│           ├── auto_ticker.go           goroutine: AdvanceTick → broadcast
│           ├── sse.go                   GET /events: 初期状態 + tick イベント配信
│           ├── static.go                webdist/ を embed.FS で配信
│           └── webdist/                 React build 出力 (.gitignore で placeholder のみ追跡)
```

### Web UI / リアルタイム同期

- `EventSource` で `/events` を購読 → tick ごとに React state 更新
- 複数タブが同じ broadcaster を購読しているので状態は同期する
- `pnpm run build` の outDir は `../internal/interface/http/server/webdist/`（embed の制約でパッケージ配下にしか置けない）
- 未ビルド時の placeholder は `webdist/index.html`（コミット済み）
- TypeScript 型は **`docs/openapi.yaml` から `openapi-typescript` で生成**（`web/src/api/schema.d.ts`、git 管理外）。`web/src/types.ts` は再 export のみ、API 呼び出しは `openapi-fetch` 経由で path/body/response が型安全。
- 仕様変更フロー: `docs/openapi.yaml` → `go generate ./...`（Go 側）+ `pnpm run build`（フロント側、build 内で `gen:api` が自動実行）

## 一次ソース

| 内容 | 場所 |
|---|---|
| API 契約 | `docs/openapi.yaml`（ハンドラ実装前に必ず確認） |
| ドメイン設計 | `docs/domain.md` |
| 振る舞い仕様 | `docs/behavior.md`（状態遷移表・配車ルール・tick セマンティクス） |
| テスト指針 | `docs/test-cases.md` |
| API 解説 | `docs/api.md` |

仕様変更時は最低でも `openapi.yaml` / `domain.md` / `behavior.md` の整合を取ること。

## 主要コマンド

```bash
go run .                       # :8080 で起動。Swagger UI は /docs
go test ./...                  # 全テスト
go vet ./...                   # 標準静的解析
golangci-lint run ./...        # lint（設定: .golangci.yml、v2 形式）
golangci-lint fmt ./...        # gofmt + goimports を当てる
go generate ./...              # OpenAPI から型・サーバ再生成
cd web && pnpm run build       # フロントビルド（webdist/ に出力、go embed 経由で配信）
```

起動時 env: `ADDR`, `TICK_INTERVAL_MS`, `FLOOR_MIN`, `FLOOR_MAX`, `ELEVATOR_COUNT`, `LOG_FORMAT`, `LOG_DEBUG`

## 設計判断（コードに残せない判断）

- **配車**: `NearestAvailableElevatorPolicy`。距離 → idle 優先 → ElevatorID 昇順で決定論。
- **冪等性**: `(floor, direction)` ごとに active な HallCall は 1 つだけ。重複ホール呼びは既存を 200 で返却、新規は 201。
- **dispatch 失敗時**: 全号機停止などで割当不能なら call を登録しない（無副作用）。HTTP は 409 `INVALID_STATE`。
- **ドア**: MVP は `open` / `closed` のみ。`opening` / `closing` は OpenAPI 上の enum に残してあるが返さない。
- **tick の 2 段階**: AdvanceOneTick は「各号機を進める → 開扉中の階に対応する assigned call を served にする」の順。多重遷移を避けるためこの順序。
- **Locker は単一**: 全 UseCase が同じ instance を共有。tick とリクエストの interleave を防ぐ。

## やらないこと

- 未実装機能を「実装した」と書かない。`oapi.Unimplemented` が 501 を返している箇所を上書きしていないなら未実装。
- 不要な抽象を増やさない（型 alias、薄い wrapper interface など）。実際にやらかして user から「indirection が読みにくい」と言われたことがある。
- 防御的分岐を増やさない。集約 / Port が保証する不変条件を信用する（ドメイン内で意図的に残した「防御的:」コメント付き分岐は許容）。
- 既存の方針を変える前にユーザに相談する。
