---
name: sync-openapi
description: docs/openapi.yaml を変更した後の再生成・再ビルド・ドキュメント整合の定型手順を実行する。API 契約（エンドポイント・スキーマ・enum）を追加/変更/削除したときに必ず使う。
---

# OpenAPI 変更後の同期手順

`docs/openapi.yaml` が唯一の API 契約。変更したら以下を順に実行し、抜けがあれば完了報告しない。

## 手順

1. **Go 側の再生成**

   ```bash
   go generate ./...
   ```

   `internal/interface/http/oapi/api.gen.go` が更新される。生成コードは編集禁止。

2. **新規エンドポイントを実装する場合のみ**: `internal/interface/http/server/handler.go` にメソッドを追加する。
   実装しないなら何もしない（`oapi.Unimplemented` が 501 を返す。501 のままなら「未実装」と報告する）。
   エラー → HTTP 変換は `errors.go` の `errorMapping` にだけ追加する。

3. **フロント側の型再生成 + ビルド**

   ```bash
   cd web && pnpm run build
   ```

   build 内で `gen:api`（openapi-typescript）が自動実行され、`web/src/api/schema.d.ts` が更新される。
   型エラーが出たら UI 側（`web/src/`)の追随が必要。

4. **ドキュメント整合**: 以下の 3 点の記述が openapi.yaml と矛盾しないか確認し、必要なら更新する。
   - `docs/domain.md`（ドメインモデルに影響する変更のとき）
   - `docs/behavior.md`（状態遷移・配車・tick セマンティクスに影響するとき)
   - `docs/api.md`（エンドポイントの追加/削除/挙動変更のとき）

5. **検証**

   ```bash
   make check          # go build + vet + test
   ```

   HTTP の挙動が変わったら `go run .` で起動し、curl で実際のレスポンスまで確認する（CLAUDE.md ルール7）。

## チェックリスト

- [ ] `go generate ./...` 実行済み（api.gen.go が openapi.yaml と同期）
- [ ] `cd web && pnpm run build` 成功（schema.d.ts 同期 + tsc 通過）
- [ ] domain.md / behavior.md / api.md に矛盾なし
- [ ] `make check` green
- [ ] 挙動変更時: curl で実レスポンス確認済み
