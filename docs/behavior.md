# Elevator 振る舞い仕様

`docs/api.md` (API) / `docs/domain.md` (ドメイン構造) の補完。実装前に確定すべき**動的な振る舞い**を定義する。

> 既出の項目（エラー → HTTP マッピング、UseCase DTO、Repository、VisibleStatus、Domain Event）は本書では繰り返さない。

## 1. 状態遷移表

エレベーターは状態機械なので、コードより先にここを固定する。

### 1.1 Elevator (操作 + tick)

「現在状態」は `(operationState, direction, doorState, stopSchedule.IsEmpty())` の組合せで考える。

| 現在状態                                     | イベント            | 次状態                                 | 備考                  |
|--------------------------------------------|-------------------|--------------------------------------|---------------------|
| `running, idle, closed, empty`               | `AddDestination(f)` (`f != current`) | `running, up/down, closed, {f}`         | `direction` を `f` の方向に決める |
| `running, idle, closed, empty`               | `AddDestination(f)` (`f == current`) | `running, idle, open, empty`            | 即時開扉              |
| `running, *, closed, !empty`                 | `tick`            | `running, up/down, closed, !empty` で 1 階移動 | 到着前              |
| `running, *, closed, !empty`                 | `tick` (移動後到着) | `running, *, open, schedule\\{f}`        | 開扉、停止予定から削除   |
| `running, *, open, *`                        | `tick`            | `running, *, closed, *`                 | MVP は 1 tick で閉。schedule 空なら同 tick で direction も idle に戻す（出発方向コミット解除） |
| `running, *, closed, empty`                  | `tick`            | `running, idle, closed, empty`          | 待機                |
| `stopped`                                    | `tick` / 操作      | 変化なし                                | 一切受け付けない         |
| `maintenance`                                | 操作              | エラー                                  | API 層で 409         |

### 1.2 DoorState

MVP では 2 値（`open`/`closed`）。後でリッチ化する場合は同じ表に `opening`/`closing` を追加する。

```text
closed → open      (到着または同階指定で開扉)
open   → closed    (次 tick で自動閉扉)
```

### 1.3 HallCall

```text
waiting  → assigned  (DispatchPolicy で割当)
assigned → served    (割当エレベーターが該当階で開扉)
waiting  → canceled  (DELETE /hall-calls/{id})
assigned → canceled  (同上)
served   → (不変)
```

MVP では `PressHallButton` 内で即時 `assigned` まで進めるため、`waiting` は永続化されない（割当不可なら呼び自体を作らずエラー）。

serve 判定では方向は見ない。「同号機が同階の up と down の両方を抱える」事態は
`dispatchCandidates` のフィルタ（§3.1）で防いでおり、serve 側で重ねて方向を
ガードすると、SCAN 配車で号機が上昇途中に逆方向呼びの階を通過したとき call が
詰まる（実質「単独 down 呼びを 1 号機で取れない」regression を生む）。

---

## 2. 停止順序（StopSchedule の解釈）

`StopSchedule` は集合（重複排除のみ）。**「どの階を次に目指すか」は `Elevator` が現在方向と組み合わせて判定する**。

### 採用: 進行方向優先（SCAN ライク）

```text
direction == up   なら、現在階より上の最も近い停止階
direction == down なら、現在階より下の最も近い停止階
その方向に予定がなくなったら反転して反対側の最遠/最近を選ぶ
direction == idle なら、距離最小の停止階に向け方向を決定
```

### 例

```text
current=3, direction=up, schedule={2, 5, 7}
  next → 5 → 7 → 2 (反転後)

current=8, direction=down, schedule={3, 6, 10}
  next → 6 → 3 → 10 (反転後)

current=5, direction=idle, schedule={2, 7}
  next → 7 (距離 2、idle なら近い方) → 2
```

### 実装メモ

`Elevator.nextFloor()` でこのロジックを担う。`StopSchedule.Floors()` は順序非保証なので、選択時に `direction` と `currentFloor` を見てソートする。

---

## 3. 配車ポリシー: NearestAvailableElevatorPolicy

### 3.1 候補フィルタ（policy 適用前に集約側で除外）

同階で逆方向の呼びを既に背負っている号機は候補から外す。「上ボタンで開いた
号機が続けて下ボタンも飲み込む」のを防ぐ。

1. その号機に **同階の逆方向 active call** が既にアサインされている → 除外
2. その号機が **その階で扉開きかつ逆方向にコミット中**（idle 同階受け入れ後の
   `direction` が反対方向）→ 除外

候補が空になったら policy は `ErrNoAvailableElevator` を返す（API 層で 409）。
1 号機運用で同階の up/down を同時押しした場合の 2 回目はこれで弾かれる。
扉が閉じれば `direction == idle` に戻るので（§1.1）、次 tick 以降に押し直せば取れる。

### 3.2 選択ルール（順に適用）

1. `operationState == running` のエレベーターのみ候補
2. 進行方向と呼び方向が整合する号機を優先（`directionCompatible`）
3. `|currentFloor - call.floor|` が最小のものを選ぶ
4. 同距離が複数 → `direction == idle` を優先
5. それでも複数 → `ElevatorID` の文字列昇順で先頭
6. 候補なし → `ErrNoAvailableElevator`

### 3.3 出発方向のコミット

idle 号機が同階の hall call を受け、`AddDestination(current)` が即時開扉した
タイミングで、号機の `direction` を呼び方向に確定する。これがその後の §3.1 の
2 番目のフィルタの判定材料になる。

### 3.4 考慮しないこと（MVP）

- 既存の `StopSchedule` 量（混雑度）
- ドア開閉中の号機の不利

---

## 4. Tick セマンティクス

`POST /simulation/tick` 1 回 = 「1 状態遷移」。

### 4.1 1 tick で起きる順序

```text
1. 各 Elevator について Elevator.AdvanceOneTick() を実行
   - operationState != running → no-op
   - doorState == open         → 閉扉して終了（移動しない）
   - schedule empty            → idle にして終了
   - それ以外                    → 次目的階方向に 1 階移動。到着なら schedule から削除して開扉
2. ElevatorBank.markServedHallCalls()
   - assigned 状態の HallCall について、割当号機が同階で open → served
```

> ドア開閉と移動が同 tick で起きないことが重要（開扉直後に動かない）。

### 4.2 各操作のコスト

| 操作         | tick 数 |
|------------|--------|
| 1 階移動      | 1      |
| 開扉         | 0（到着 tick で同時に open） |
| 開状態の維持   | 1      |
| 閉扉         | 0（次 tick で即 closed） |

### 4.3 tick の進行方法

| 方式               | MVP   | 備考                                |
|------------------|------|-----------------------------------|
| API 手動 (`POST /simulation/tick`) | ✅   | テスト・デバッグ容易                  |
| 一定間隔の自動進行         | ✗   | 後続。`Clock` インターフェース経由で導入        |

---

## 5. Building / Floor 制約（既定値）

`docs/domain.md` §5 の `BuildingSpec` に対する MVP の既定値と仕様判断。

| 項目                      | MVP 既定                |
|-------------------------|------------------------|
| `minFloor`              | `1`                     |
| `maxFloor`              | `10`                    |
| 地下階                    | 扱わない（VO 上は表現可能、`BuildingSpec` で弾く） |
| `floor == 0`             | 有効性は `BuildingSpec.Contains` のみが判断する（MVP 既定 `1..10` では暗黙に範囲外として弾かれる）。0 階を除外したい建物は `BuildingSpec` 側に「0 を含めない」バリエーションを後で導入する。 |
| 全エレベーターが全階に停止可能         | はい                     |
| スキップフロア / 専用号機         | なし                     |
| 最上階で `up` ホール呼び         | `ErrInvalidHallCallDirection` |
| 最下階で `down` ホール呼び        | `ErrInvalidHallCallDirection` |
| 現在階を行き先指定 (Car Call)    | 即時開扉（成功扱い）         |
| 現在階を行き先指定 (Hall Call)   | 通常通り受付（同階に止まる） |

> 既存コード `main.go` の `FloorRange{Min:-2, Max:20}` は地下対応のサンプル値。MVP の domain default は 1〜10 にし、`Reset` で上書き可能にする。

---

## 6. 並行制御

複数リクエストが同時に到着すると、`ElevatorBank` の状態が壊れる。MVP のメモリ実装では Repository 層で粗いロックをかける。

### 6.1 MVP（インメモリ）

```text
ElevatorBankRepository の実装が sync.Mutex を持ち、
Find→処理→Save の間を UseCase 側でロック区間にする。
```

実装上のシンプルな案: Repository ではなく UseCase 側に注入される `Locker` を用意し、各 UseCase が `Lock → Find → 処理 → Save → Unlock` を強制する。

### 6.2 将来（DB 永続化）

`ElevatorBank` に `version int` を持たせ、`Save` で楽観ロック (`UPDATE ... WHERE version = ?`)。失敗したら UseCase で再試行 or 409 を返す。

---

## 7. ID 生成

ドメインに直接 UUID 生成を書かず、`IDGenerator` をインターフェース化してテストで差し替え可能にする。

```go
// internal/domain/elevator/id.go (or usecase 側)
type IDGenerator interface {
    NewID() string
}
```

| ID 種別        | 生成方針                       |
|--------------|----------------------------|
| `ElevatorID`  | 設定 / Reset 時に固定（`ev-1`, `ev-2`） |
| `HallCallID`  | UUID v4 を `IDGenerator` で生成 |

UseCase は `IDGenerator` を依存に持ち、`PressHallButtonUseCase` でのみ採番する。テストは固定値を返す `FakeIDGenerator` を使う。

---

## 8. 初期設定 / Reset

`POST /simulation/reset` の入力で全状態を作り直す。設定構造体は `domain` に置かず、UseCase 入力 DTO とする。

```go
type ResetSimulationInput struct {
    MinFloor      int
    MaxFloor      int
    Elevators     []ElevatorInit
}

type ElevatorInit struct {
    ID            string
    InitialFloor  int
}
```

### MVP デフォルト（リクエスト省略時）

```text
minFloor = 1
maxFloor = 10
elevators:
  - id: ev-1, initialFloor: 1
  - id: ev-2, initialFloor: 10
```

`doorOpenTicks` / `moveTicksPerFloor` は MVP では各 1 で固定し、設定化しない。リッチ化時に `SimulationConfig` を導入する。

---

## 9. 実装着手順

`docs/api.md` §4.1 の MVP 4 本を作るための、内側からの実装順。

1. **VO** (`Floor`, `Direction`, `DoorState`, `OperationState`, `HallCallStatus`, `BuildingSpec`)
2. **`StopSchedule`** + `nextFloor` ロジック（§2）
3. **`Elevator` Entity** + `AdvanceOneTick` の状態遷移（§1.1, §4.1）
4. **`HallCall` Entity** (§1.3)
5. **`DispatchPolicy` / `NearestAvailableElevatorPolicy`** (§3)
6. **`ElevatorBank`** Aggregate Root（`PressHallButton` / `PressCarButton` / `AdvanceOneTick` / `VisibleElevatorsFrom`）
7. **`ElevatorBankRepository`** (memory + mutex, §6.1)
8. **UseCase 4 本** (`PressHallButton` / `PressCarButton` / `AdvanceTick` / `GetVisibleElevators`)
9. **HTTP Handler / Router**

各ステップで `docs/test-cases.md` のテストを先に書く（TDD）。
