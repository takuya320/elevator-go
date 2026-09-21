# Elevator API 仕様書

エレベーターシミュレータの REST API 仕様。ドメイン層（`internal/domain/elevator`）の `ElevatorBank` 集約を HTTP で公開する。

## 1. 概要

### スコープ

- 利用者向け: フロアからのエレベーター状態参照、呼び出しボタン押下、車内行き先指定
- 管理向け: エレベーターの追加・状態更新・停止/再開・ドア操作
- シミュレーション: 1tick 進行、状態リセット

### 設計方針

- パスは複数形リソース + サブリソースで統一（`/elevators/{id}/car-calls` 等）
- 階番号 `floor` は整数。地下階は負値で表現（例: `B2 = -2`、`1F = 1`）
- 状態は文字列 enum（後述）
- すべての時刻は RFC 3339 (UTC) 文字列
- `Content-Type: application/json` 固定

### 共通エラーレスポンス

```json
{
  "error": {
    "code": "OUT_OF_RANGE",
    "message": "floor 25 is outside [-2, 20]"
  }
}
```

| HTTP | code                  | 発生条件                                       |
|------|-----------------------|------------------------------------------|
| 400  | `INVALID_REQUEST`     | JSON 不正、必須フィールド欠落                          |
| 400  | `OUT_OF_RANGE`        | 階が `FloorRange` 外                          |
| 400  | `INVALID_DIRECTION`   | 最上階で `up` / 最下階で `down` / 不正な向き            |
| 404  | `ELEVATOR_NOT_FOUND`  | `elevatorId` 未存在                          |
| 404  | `CALL_NOT_FOUND`      | `callId` 未存在                              |
| 409  | `INVALID_STATE`       | 点検中エレベーターへの操作など状態的に不可な操作                   |
| 500  | `INTERNAL`            | 想定外                                       |

---

## 2. 共通型 (enum)

Go の定数定義と 1:1 対応。

### Direction

| 値        | 意味              | Go 定数        |
|-----------|-----------------|--------------|
| `up`      | 上昇 / 上方向呼び出し     | `DirUp`      |
| `down`    | 下降 / 下方向呼び出し     | `DirDown`    |
| `idle`    | 停止中（待機）         | `DirNone`    |

### DoorState

| 値          | 意味     |
|-------------|--------|
| `open`      | 開      |
| `opening`   | 開動作中   |
| `closed`    | 閉      |
| `closing`   | 閉動作中   |

> レスポンスに現れるのは `open` / `closed` のみ。`opening` / `closing` は enum に定義してあるが返さない。ドアの多段遷移を実装するときに有効化する。

### OperationState

| 値              | 意味        |
|-----------------|-----------|
| `running`       | 通常運転中     |
| `stopped`       | 停止中       |
| `maintenance`   | 点検中       |

### HallCallStatus

| 値          | 意味              |
|-------------|-----------------|
| `waiting`   | 受付済み・未割当         |
| `assigned`  | エレベーター割当済み      |
| `served`    | 到着済み（応答完了）     |
| `canceled`  | 取り消し            |

---

## 3. リソースモデル

### Elevator

| フィールド             | 型               | 備考                              |
|----------------------|-----------------|---------------------------------|
| `id`                 | string          | 例: `ev-1`                       |
| `currentFloor`       | int             | 現在階                            |
| `direction`          | Direction       |                                 |
| `doorState`          | DoorState       |                                 |
| `operationState`     | OperationState  |                                 |
| `floorRange`         | object          | 既定は `{ "min": 1, "max": 10 }`。地下は負値 |
| `destinationFloors`  | int[]           | 車内行き先（点灯中ボタン）              |
| `assignedHallCalls`  | HallCall[]      | 自エレベーターに割当済みのホール呼び       |
| `doorHoldOpen`       | bool            | 「開」ボタンによる保持中。true の間は移動・自動閉扉なし |
| `homeFloor`          | int             | 自動帰還の対象階。既定は `initialFloor`   |
| `autoReturnEnabled`  | bool            | 自動帰還のオン/オフ                    |

### HallCall

| フィールド               | 型               | 備考                       |
|-------------------------|-----------------|--------------------------|
| `id`                    | string          | 例: `call-123`             |
| `floor`                 | int             |                          |
| `direction`             | `up` \| `down`  | `idle` は不可               |
| `status`                | HallCallStatus  |                          |
| `assignedElevatorId`    | string \| null  | 未割当時は null               |
| `createdAt`             | string (RFC3339)|                          |

---

## 4. エンドポイント一覧

### 4.1 利用者向けの中心 4 本

| Method | Path                                  | 用途                       |
|--------|---------------------------------------|--------------------------|
| GET    | `/floors/{floor}/elevators`           | n 階から見えるエレベーター状態        |
| POST   | `/floors/{floor}/hall-calls`          | フロアの上下ボタン押下             |
| POST   | `/elevators/{elevatorId}/car-calls`   | 車内の行き先ボタン押下             |
| POST   | `/simulation/tick`                    | シミュレーションを 1 ステップ進める   |

### 4.2 全エンドポイント

OpenAPI 上の 16 operation はすべて実装済み。`Handler` は `oapi.Unimplemented` を
埋め込まないので、生成後にメソッドを取りこぼすとコンパイルエラーになる。

```text
GET    /elevators
POST   /elevators
GET    /elevators/{elevatorId}
PATCH  /elevators/{elevatorId}

POST   /elevators/{elevatorId}/car-calls
POST   /elevators/{elevatorId}/doors/open
POST   /elevators/{elevatorId}/doors/close
POST   /elevators/{elevatorId}/stop
POST   /elevators/{elevatorId}/resume

GET    /floors/{floor}/elevators
GET    /floors/{floor}/hall-calls
POST   /floors/{floor}/hall-calls

GET    /hall-calls
DELETE /hall-calls/{callId}

POST   /simulation/tick
POST   /simulation/reset
```

OpenAPI に定義のない配信系が 3 本ある。

```text
GET    /               React UI（embed 済み）
GET    /events         SSE。接続直後に現在状態、以降は tick ごとに全状態
GET    /docs           Swagger UI（仕様は GET /openapi.json）
```

---

## 5. 利用者向け API

### 5.1 GET `/floors/{floor}/elevators`

n 階にいる人から見えるエレベーター一覧。

**Path**
- `floor` (int, 必須) — `FloorRange` 内

**Response 200**
```json
{
  "floor": 5,
  "elevators": [
    {
      "id": "ev-1",
      "currentFloor": 3,
      "direction": "up",
      "doorState": "closed",
      "operationState": "running",
      "visibleStatus": "approaching"
    }
  ]
}
```

`visibleStatus` は階表示盤に出す派生情報。取りうる値は次の 5 種:

| 値              | 意味                                              |
|---------------|-------------------------------------------------|
| `arrived`     | 同階に停止中でドアが開いている                                   |
| `approaching` | 自階に向かって接近中                                       |
| `passing`     | 同階だが停止せず通過中／同階で扉が閉じている移動中                       |
| `away`        | 自階から離れている（接近もしていない）                              |
| `unavailable` | 該当号機が `running` でない（`stopped` / `maintenance` 中） |

---

### 5.2 POST `/floors/{floor}/hall-calls`

ホール呼び（上下ボタン）を登録する。`PressHallButton` 相当。

**Path**
- `floor` (int, 必須)

**Request**
```json
{ "direction": "up" }
```

**Response 201**（新規受付。MVP では受付時に即時割当まで進めるため `status` は `assigned`）
```json
{
  "id": "call-123",
  "floor": 5,
  "direction": "up",
  "status": "assigned",
  "assignedElevatorId": "ev-1",
  "createdAt": "2026-05-09T10:15:00Z"
}
```

**Response 200**（同一 (floor, direction) で `waiting`/`assigned` の呼びが既存。冪等返却）
- ボディは既存 `HallCall` をそのまま返す。

**バリデーション**
- 最上階（`FloorRange.Max`）で `up` → `400 INVALID_DIRECTION`
- 最下階（`FloorRange.Min`）で `down` → `400 INVALID_DIRECTION`
- `direction` が `idle` または不正値 → `400 INVALID_DIRECTION`
- 範囲外階 → `400 OUT_OF_RANGE`
- 全号機が `stopped` / `maintenance` で割当先なし → `409 INVALID_STATE`（呼びは登録されない）

---

### 5.3 POST `/elevators/{elevatorId}/car-calls`

車内行き先ボタン押下。`SelectDestination` 相当。

**Request**
```json
{ "destinationFloor": 10 }
```

**Response 201**
```json
{
  "elevatorId": "ev-1",
  "destinationFloor": 10,
  "status": "accepted"
}
```

**バリデーション**
- `destinationFloor` が `FloorRange` 外 → `400 OUT_OF_RANGE`
- `destinationFloor == currentFloor` → `201`（schedule には積まず即時開扉。`docs/behavior.md` §5 参照）
- `operationState != running` → `409 INVALID_STATE`

---

## 6. 管理 API

### 6.1 GET `/elevators`

全エレベーター一覧。`ElevatorID` の昇順。

**Response 200**
```json
{ "elevators": [ /* Elevator[] */ ] }
```

### 6.2 POST `/elevators`

エレベーター追加（シミュレーター用）。追加した号機は次の配車から候補に入る。

**Request**
```json
{
  "id": "ev-3",
  "initialFloor": 1
}
```

階範囲は**建物共通**（`BuildingSpec`）で号機ごとには持たない。スキップフロア・専用号機は
`docs/behavior.md` §5 でスコープ外としているため、リクエストで号機ごとの `floorRange` は
受け付けない。範囲を変えるときは `POST /simulation/reset`。

`homeFloor` は `initialFloor` を引き継ぎ、`autoReturnEnabled` は false で始まる。変更は
`PATCH /elevators/{id}`。

**Response 201**: `Elevator`

| 条件                     | ステータス | code               |
|------------------------|--------|--------------------|
| `id` が既存と重複          | 400    | `INVALID_REQUEST`  |
| `id` が空                | 400    | `INVALID_REQUEST`  |
| `initialFloor` が範囲外    | 400    | `OUT_OF_RANGE`     |

### 6.3 GET `/elevators/{elevatorId}`

詳細取得。**Response 200**: `Elevator` / 未知の ID は 404 `ELEVATOR_NOT_FOUND`。

### 6.4 PATCH `/elevators/{elevatorId}`

状態の手動更新（テスト・管理用）。

**Request**（すべて任意、与えたフィールドのみ更新）
```json
{
  "currentFloor": 4,
  "direction": "up",
  "doorState": "closed",
  "operationState": "running",
  "homeFloor": 1,
  "autoReturnEnabled": true
}
```

`doorState` は `open` / `closed` のみ受け付ける。`currentFloor` / `homeFloor` の範囲検証は集約側で行い、外れていれば 400 `OUT_OF_RANGE`。

**Response 200**: `Elevator`

### 6.5 ドア操作

- `POST /elevators/{elevatorId}/doors/open` → `doorState` を `open`、`doorHoldOpen` を `true`。保持中は移動も自動閉扉もしない
- `POST /elevators/{elevatorId}/doors/close` → `doorState` を `closed`、`doorHoldOpen` を `false`。次 tick から通常運行に戻る

どちらも dwell（開扉を保持する tick 数）を 0 に戻すので、「開」直後の「閉」が即時に効く。到着時の自動開扉と dwell の関係は `docs/behavior.md` §1.2。

**Response 200**: `Elevator`

### 6.6 運転制御

- `POST /elevators/{elevatorId}/stop` → `operationState` を `stopped`
- `POST /elevators/{elevatorId}/resume` → `operationState` を `running`

`stopped` の間は tick で一切進まず、かご内行先の追加も 409 `INVALID_STATE`。schedule と direction は保持され、`resume` で続きから動く。

**Response 200**: `Elevator`

---

## 7. ホール呼び一覧 API

### 7.1 GET `/hall-calls`

保持している全呼びからの横断検索。**`served` / `canceled` も消さずに残している**ので、
無指定だとシミュレーション開始以降の全件が返る。点灯中のボタンだけが欲しいなら
`?status=waiting,assigned`。

**Query**
- `status` (任意): `waiting` / `assigned` / `served` / `canceled`（カンマ区切り可）。
  未知の値は 400 `INVALID_REQUEST`
- `floor` (任意): int。ここは絞り込みなので範囲外でもエラーにせず 0 件を返す

**Response 200**: `{ "hallCalls": HallCall[] }`（`HallCallID` の昇順）

### 7.2 GET `/floors/{floor}/hall-calls`

特定階の呼び一覧（status での絞り込みなし）。**Response 200**: `{ "floor": 5, "calls": HallCall[] }`

こちらは階そのものがリソースなので、範囲外の階は空配列ではなく 400 `OUT_OF_RANGE`。
§7.1 の `?floor=` が絞り込みなのと扱いが違う。

### 7.3 DELETE `/hall-calls/{callId}`

呼びをキャンセル（管理・シミュレーター用途のみ）。

**Response 204** No Content

---

## 8. シミュレーション API

### 8.1 POST `/simulation/tick`

時間を 1 ステップ進め、各エレベーターを 1 階分（またはドア状態 1 段階）動かす。
`GET /events`（SSE）も同じ形を配信する。

**Response 200**
```json
{
  "tick": 42,
  "elevators": [ /* Elevator[] */ ],
  "hallCalls": [ /* HallCall[]：active（waiting / assigned）な呼び */ ],
  "events": [ /* SimulationEvent[] */ ]
}
```

`hallCalls` はホールボタンの点灯状態に対応する。割当先の号機が停止すると呼びは
`waiting` に戻って `Elevator.assignedHallCalls` から外れるため、点灯の判定には
号機側ではなく必ずこちらを使う（`docs/behavior.md` §3.4）。

`events` の `hall_call.reassigned` は呼びの割当先が変わったことを表す。`elevatorId` が
付いていれば新しい割当先、省略されていれば引き受けられる号機が無く `waiting` に
戻ったことを意味する。

### 8.2 POST `/simulation/reset`

全状態を破棄し、与えた設定で初期化。

**Request**
```json
{
  "floorRange": { "min": 1, "max": 10 },
  "elevators": [
    { "id": "ev-1", "initialFloor": 1 },
    { "id": "ev-2", "initialFloor": 10 }
  ]
}
```

各フィールドは省略可。省略時は起動時 env から組み立てた既定値を使う（`docs/behavior.md` §8）:

- `floorRange` 省略時: `FLOOR_MIN` / `FLOOR_MAX`（既定 `{ "min": 1, "max": 10 }`）
- `elevators` 省略時: `ELEVATOR_COUNT` 台（既定 2 台）を階範囲に等間隔配置し、`ev-1`, `ev-2`, … と採番

body を空（`Content-Length: 0`）で呼んでも同じ既定値が適用される。UI のリセットボタンと起動時の初期化はこの経路を使う。

**Response 200**
```json
{
  "status": "reset",
  "floorRange": { "min": 1, "max": 10 },
  "elevators": [
    { "id": "ev-1", "initialFloor": 1 },
    { "id": "ev-2", "initialFloor": 10 }
  ]
}
```

---

## 9. ドメイン対応表

`ElevatorBank` 集約（`internal/domain/elevator`）とエンドポイントの対応。

| ドメイン API                      | HTTP                                                          |
|---------------------------------|---------------------------------------------------------------|
| `Elevator.CurrentFloor`          | `Elevator.currentFloor`（tick / SSE / 可視一覧のレスポンス）          |
| `ElevatorBank.Spec`              | `Elevator.floorRange`                                         |
| `ElevatorBank.PressHallButton`   | `POST /floors/{floor}/hall-calls`                             |
| `ElevatorBank.HallCalls`         | `Elevator.assignedHallCalls` → `status != served` で点灯判定      |
| `ElevatorBank.PressCarButton`    | `POST /elevators/{id}/car-calls`                              |
| `Elevator.Destinations`          | `Elevator.destinationFloors` に含まれるか = かご内ボタン点灯          |
| `ElevatorBank.OpenDoor` / `CloseDoor` | `POST /elevators/{id}/doors/open` ・ `.../doors/close`    |
| `ElevatorBank.CancelHallCall`    | `DELETE /hall-calls/{callId}`                                 |
| `ElevatorBank.AdvanceOneTick`    | `POST /simulation/tick`、および auto-ticker → `GET /events`      |
| `ElevatorBank.VisibleElevatorsFrom` | `GET /floors/{floor}/elevators`                            |
| `ElevatorBank.Elevators` / `Elevator` | `GET /elevators` ・ `GET /elevators/{id}`                  |
| `ElevatorBank.AddElevator`       | `POST /elevators`                                             |
| `ElevatorBank.HallCalls`         | `GET /hall-calls` ・ `GET /floors/{floor}/hall-calls`          |

---

## 10. 実装メモ

- ボタン点灯解除は運行側で制御（`SelectDestination` で点灯 → 到着で消灯、ホール呼びは応答時に `served` に遷移して消灯）
- ドア開閉は `open` / `closed` の 2 値のまま、開扉の次の tick を dwell として挟むことで「開いた瞬間に閉まる」のを避けている（`docs/behavior.md` §1.2）。`opening` / `closing` を使った多段遷移は未実装
- 並行アクセスは `usecase.Locker`（`internal/infrastructure/sync` の単一 mutex）で直列化する。全 UseCase が同じ instance を共有し、tick とリクエストの interleave を防ぐ
