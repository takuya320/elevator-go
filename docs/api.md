# Elevator API 仕様書

エレベーターシミュレータの REST API 仕様。Go 実装（`elevator/elevator.go`）の `Elevator` インターフェースを HTTP で公開する想定。

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

> **MVP**: レスポンスに現れるのは `open` / `closed` のみ。`opening` / `closing` は将来のドア多段遷移実装時に有効化する。

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
| `floorRange`         | object          | `{ "min": -2, "max": 20 }`      |
| `destinationFloors`  | int[]           | 車内行き先（点灯中ボタン）              |
| `assignedHallCalls`  | HallCall[]      | 自エレベーターに割当済みのホール呼び       |

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

### 4.1 MVP（最初に実装する 4 本）

| Method | Path                                  | 用途                       |
|--------|---------------------------------------|--------------------------|
| GET    | `/floors/{floor}/elevators`           | n 階から見えるエレベーター状態        |
| POST   | `/floors/{floor}/hall-calls`          | フロアの上下ボタン押下             |
| POST   | `/elevators/{elevatorId}/car-calls`   | 車内の行き先ボタン押下             |
| POST   | `/simulation/tick`                    | シミュレーションを 1 ステップ進める   |

### 4.2 全エンドポイント

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

全エレベーター一覧。

**Response 200**
```json
{ "elevators": [ /* Elevator[] */ ] }
```

### 6.2 POST `/elevators`

エレベーター追加（シミュレーター用）。

**Request**
```json
{
  "id": "ev-3",
  "initialFloor": 1,
  "floorRange": { "min": -2, "max": 20 }
}
```

`floorRange` を省略した場合、システムデフォルト値を使用。

**Response 201**: `Elevator`

### 6.3 GET `/elevators/{elevatorId}`

詳細取得。**Response 200**: `Elevator`

### 6.4 PATCH `/elevators/{elevatorId}`

状態の手動更新（テスト・管理用）。

**Request**（すべて任意、与えたフィールドのみ更新）
```json
{
  "currentFloor": 4,
  "direction": "up",
  "doorState": "closed",
  "operationState": "running"
}
```

### 6.5 ドア操作

> **MVP スコープ外**。ドアの多段遷移 (`opening`/`closing`) と併せて将来実装する。

- `POST /elevators/{elevatorId}/doors/open` → `doorState` を `opening` → `open`
- `POST /elevators/{elevatorId}/doors/close` → `doorState` を `closing` → `closed`

### 6.6 運転制御

- `POST /elevators/{elevatorId}/stop` → `operationState` を `stopped`
- `POST /elevators/{elevatorId}/resume` → `operationState` を `running`

---

## 7. ホール呼び一覧 API

### 7.1 GET `/hall-calls`

**Query**
- `status` (任意): `waiting` / `assigned` / `served` / `canceled`（カンマ区切り可）
- `floor` (任意): int

**Response 200**: `{ "hallCalls": HallCall[] }`

### 7.2 GET `/floors/{floor}/hall-calls`

特定階の呼び一覧。**Response 200**: `{ "floor": 5, "calls": HallCall[] }`

### 7.3 DELETE `/hall-calls/{callId}`

呼びをキャンセル（管理・シミュレーター用途のみ）。

**Response 204** No Content

---

## 8. シミュレーション API

### 8.1 POST `/simulation/tick`

時間を 1 ステップ進め、各エレベーターを 1 階分（またはドア状態 1 段階）動かす。

**Response 200**
```json
{
  "tick": 42,
  "elevators": [ /* Elevator[] */ ]
}
```

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

各フィールドは省略可。省略時は MVP 既定値を使用する（`docs/behavior.md` §8）:

- `floorRange` 省略時: `{ "min": 1, "max": 10 }`
- `elevators` 省略時: `ev-1@minFloor` と `ev-2@maxFloor` の 2 台

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

## 9. インターフェース対応表

既存 Go インターフェース (`elevator.Elevator`) とエンドポイントの対応。

| Go メソッド             | HTTP                                                    |
|-----------------------|---------------------------------------------------------|
| `CurrentFloor`        | `GET /elevators/{id}` → `currentFloor`                  |
| `Range`               | `GET /elevators/{id}` → `floorRange`                    |
| `PressHallButton`     | `POST /floors/{floor}/hall-calls`                       |
| `IsHallButtonLit`     | `GET /floors/{floor}/hall-calls` → `status != served`   |
| `SelectDestination`   | `POST /elevators/{id}/car-calls`                        |
| `IsCarButtonLit`      | `GET /elevators/{id}` → `destinationFloors` に含まれるか    |

---

## 10. 実装メモ

- ボタン点灯解除は運行側で制御（`SelectDestination` で点灯 → 到着で消灯、ホール呼びは応答時に `served` に遷移して消灯）
- `tick` ベースで時間進行する場合、ドア開閉は複数 tick にまたがる状態遷移として扱う（`opening` → `open` → `closing` → `closed`）
- 並行アクセスはサーバ側でロックする前提（現状の `InMemoryElevator` は未対応のため、HTTP 層を作る前にミューテックスを入れる必要あり）
