# ドメインテストケース

`docs/behavior.md` の振る舞い仕様を担保するテスト一覧。実装より先に書く（TDD）。

> Handler / UseCase の入出力テストは別途。本書はドメイン層 (`internal/domain/elevator`) のみ対象。

## 1. Value Object

### `Floor`
- `Above()` / `Below()` で隣接階を返す
- `Equals` の同値判定
- 負値 (`-1`) でも生成できる（範囲チェックは `BuildingSpec` 側）

### `BuildingSpec`
- `min >= max` で `NewBuildingSpec` がエラー
- `Contains` が範囲内/外を正しく判定
- `CanCall(maxFloor, up)` → `false`
- `CanCall(minFloor, down)` → `false`
- `CanCall(floor, idle)` → `false`
- `CanCall(範囲外, *)` → `false`

### `StopSchedule`
- 空の状態で `IsEmpty() == true`
- 同じ階を 2 回 `Add` しても `Floors()` の長さは 1
- `Remove` で除去できる、未登録階の `Remove` は no-op

### `Direction` / `DoorState` / `OperationState` / `HallCallStatus`
- 文字列値が API 仕様 (`docs/api.md` §2) と一致

---

## 2. `Elevator` Entity

### 初期状態
- `NewElevator(id, initial)` → `currentFloor=initial`, `direction=idle`, `doorState=closed`, `operationState=running`, `stopSchedule` 空

### `AddDestination`
- 通常追加で `stopSchedule` に入る
- 現在階と同じ階を指定 → `doorState=open`、`stopSchedule` には**入らない**
- `operationState=stopped` で呼ぶ → `ErrElevatorNotRunning`
- `operationState=maintenance` で呼ぶ → `ErrElevatorNotRunning`

### `AdvanceOneTick` (§1.1, §4.1)
- `running, idle, closed, empty` で tick → 何も変わらない
- `running, idle, closed, schedule={5}` (current=3) で tick → `currentFloor=4`, `direction=up`, `closed`
- 移動して目的階到達 → `currentFloor=5`, `door=open`, `schedule` から `5` が消える
- `door=open, dwell>0` で tick → `door=open` のまま、dwell を 1 消費して終了
- `door=open, dwell=0` で tick → `door=closed`、移動しない（同 tick で動かない）
- 自動帰還オンで `idle, closed, empty, current!=home` で tick → home を schedule に積み 1 階移動
- `stopSchedule` が空になった次の tick → `direction=idle`
- `operationState=stopped` で tick → 状態変化なし

### 停止順序 (§2)
- `current=3, direction=up, schedule={2,5,7}` → 次の到達順は `5, 7, 2`
- `current=8, direction=down, schedule={3,6,10}` → 次の到達順は `6, 3, 10`
- `current=5, direction=idle, schedule={2,7}` → 最初に向かうのは距離が近い `7`

---

## 3. `HallCall` Entity

- `direction=idle` で `NewHallCall` → `ErrInvalidHallCallDirection`
- 初期状態は `status=waiting`, `assignedElevatorID=nil`
- `AssignTo(id)` → `status=assigned`, `assignedElevatorID=id`
- `MarkServed()` → `status=served`
- `Cancel()` で `waiting`/`assigned` → `canceled`
- `Cancel()` を `served` に対して呼んでも変化しない

---

## 4. `DispatchPolicy` / `NearestAvailableElevatorPolicy` (§3)

- `running` 1 台のみ → 必ずそれを選ぶ
- 距離が異なる 2 台 → 近い方を選ぶ
- 距離同点で `idle` と `up` が混在 → `idle` を選ぶ
- 距離同点 + `direction` 同じ → `ElevatorID` 昇順で先頭
- 全台 `stopped` → `ErrNoAvailableElevator`
- `running` のうち候補が 0 台（全 stopped/maintenance）→ `ErrNoAvailableElevator`

---

## 5. `ElevatorBank` Aggregate Root

### `PressHallButton`
- 通常呼び → 新規 `HallCall` 作成、`assigned` 状態、対象号機の `stopSchedule` に階が追加される
- 同じ `(floor, direction)` で `waiting`/`assigned` がある → 既存をそのまま返し、新規作成しない
- 最上階で `up` → `ErrInvalidHallCallDirection`
- 最下階で `down` → `ErrInvalidHallCallDirection`
- `direction == idle` → `ErrInvalidHallCallDirection`
- 範囲外階 → `ErrInvalidFloor`
- 全号機 `stopped` / `maintenance` → `ErrNoAvailableElevator`、`HallCall` は登録されない（副作用なし）

### `PressCarButton`
- 通常指定 → 該当号機の `stopSchedule` に追加
- 範囲外階 → `ErrInvalidDestinationFloor`
- 存在しない `elevatorID` → `ErrElevatorNotFound`
- `operationState != running` の号機 → `ErrElevatorNotRunning`
- 現在階と同じ階指定 → 開扉のみ（成功）

### `AdvanceOneTick`
- 全号機の `AdvanceOneTick` を呼ぶ
- 割当済み `HallCall` の階で号機が開扉 → `served` に遷移
- `served` 後に再度 tick しても多重遷移しない

### `VisibleElevatorsFrom` (§ `domain.md` §10)
- 範囲外 floor → `ErrInvalidFloor`
- `operationState != running` → `unavailable`
- 同階で `door=open` → `arrived`
- `direction=up` で `currentFloor < viewer` → `approaching`
- `direction=down` で `currentFloor > viewer` → `approaching`
- 同階で移動中 / `closed` → `passing`
- それ以外 → `away`

---

## 6. シナリオテスト（結合）

ドメインのみで完結するシナリオを最低 1 本通す。

### シナリオ A: 単独号機の往復
```text
setup: building 1-10, elevator ev-1 at 1F
1. PressHallButton(5, up)        → ev-1 に割当、schedule={5}
2. tick × 4                       → ev-1: 5F, door=open, HallCall served
3. PressCarButton(ev-1, 8)        → schedule={8}
4. tick × 1                       → door=open のまま（dwell 消費）
5. tick × 1                       → door=closed
6. tick × 3                       → ev-1: 8F, door=open
```

### シナリオ B: 2 号機の振り分け
```text
setup: ev-1 at 1F, ev-2 at 10F
1. PressHallButton(8, down)       → ev-2 に割当（距離 2）
2. PressHallButton(2, up)         → ev-1 に割当（距離 1）
3. tick × N                       → 両方 served になる
```

---

## 7. テスト命名規則

```text
TestElevator_AddDestination_AddsToStopSchedule
TestElevator_AdvanceOneTick_OpensDoorOnArrival
TestElevatorBank_PressHallButton_RejectsTopFloorUp
```

`Test<対象>_<メソッド>_<期待される振る舞い>` で揃える。テーブルテストは `cases := []struct{...}` で定義する。
