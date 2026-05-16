package elevator

import "fmt"

// 到着で扉を開けた tick の後、自動閉扉までに挟む dwell の tick 数。
// AdvanceOneTick は扉開きの tick → dwell の tick → 閉扉の tick の順に進む。
const doorDwellTicks = 1

// 集約境界をまたぐ判断（範囲検証・配車）は持たない。それは ElevatorBank の責務。
// 並行不可: 集約の利用側がアクセスを直列化する前提。
type Elevator struct {
	id             ElevatorID
	currentFloor   Floor
	direction      Direction
	doorState      DoorState
	operationState OperationState
	stopSchedule   StopSchedule
	// 「開」ボタンが押された後、明示的に「閉」が押されるまで true。
	// 真である間は AdvanceOneTick が move も自動閉扉もしない。
	holdOpen bool
	// 自動閉扉前に扉を開いたまま残す残り tick 数。到着・同階指定で扉が開くたびに
	// doorDwellTicks にリセットされる。0 になった次の tick で自動閉扉する。
	doorDwell int
	// 自動帰還用の「ホーム階」。schedule が空かつ自動帰還オンで現在階と異なる時、
	// AdvanceOneTick が schedule に積み直して移動を再開する。
	homeFloor Floor
	// true のとき空き状態で homeFloor に自動的に戻る。
	autoReturnEnabled bool
}

func NewElevator(id ElevatorID, initial Floor) *Elevator {
	return &Elevator{
		id:             id,
		currentFloor:   initial,
		direction:      DirectionIdle,
		doorState:      DoorStateClosed,
		operationState: OperationStateRunning,
		stopSchedule:   NewStopSchedule(),
		homeFloor:      initial,
	}
}

func (e *Elevator) ID() ElevatorID                 { return e.id }
func (e *Elevator) CurrentFloor() Floor            { return e.currentFloor }
func (e *Elevator) Direction() Direction           { return e.direction }
func (e *Elevator) DoorState() DoorState           { return e.doorState }
func (e *Elevator) OperationState() OperationState { return e.operationState }
func (e *Elevator) IsRunning() bool                { return e.operationState == OperationStateRunning }
func (e *Elevator) HasDestination(f Floor) bool    { return e.stopSchedule.Has(f) }
func (e *Elevator) Destinations() []Floor          { return e.stopSchedule.Floors() }
func (e *Elevator) HomeFloor() Floor               { return e.homeFloor }
func (e *Elevator) AutoReturnEnabled() bool        { return e.autoReturnEnabled }
func (e *Elevator) IsAtFloorWithDoorOpen(f Floor) bool {
	return e.currentFloor.Equals(f) && e.doorState == DoorStateOpen
}

// 管理者操作: 運転状態を直接書き換える。schedule や direction は保持される。
// 不正値の検証は呼び出し側 (Bank.PatchElevator) の責務。
func (e *Elevator) SetOperationState(s OperationState) {
	e.operationState = s
}

// 開ボタン: 扉を開けて hold-open 状態へ移行する。明示的に CloseDoor が
// 呼ばれるまで AdvanceOneTick は move も自動閉扉もしない。
// hold-open の間は dwell カウントを使わないので 0 にしておく
// （閉ボタンで即時に閉じられるようにするため）。
func (e *Elevator) OpenDoor() {
	e.doorState = DoorStateOpen
	e.holdOpen = true
	e.doorDwell = 0
}

// 閉ボタン: hold-open を解除し扉を閉じる。次 tick から通常の運行に戻る。
// 残っていた dwell も即時消費したいので 0 にする。
func (e *Elevator) CloseDoor() {
	e.doorState = DoorStateClosed
	e.holdOpen = false
	e.doorDwell = 0
}

func (e *Elevator) HoldOpen() bool { return e.holdOpen }

// 到着または同階指定で扉を開けるときの共通処理。dwell を doorDwellTicks に
// リセットすることで「開いた次の tick で即閉扉」を防ぐ。
func (e *Elevator) openDoorOnArrival() {
	e.doorState = DoorStateOpen
	e.doorDwell = doorDwellTicks
}

// 自動帰還の設定。homeFloor の範囲チェックは集約 (ElevatorBank) 側で行う前提。
func (e *Elevator) setHomeFloor(f Floor)         { e.homeFloor = f }
func (e *Elevator) setAutoReturnEnabled(b bool)  { e.autoReturnEnabled = b }

// 行先 == 現在階のときは schedule に積まずに即座に開扉する。
// 範囲検証は集約 (ElevatorBank) 側で行う前提。
func (e *Elevator) AddDestination(f Floor) error {
	if !e.IsRunning() {
		return fmt.Errorf("%w: elevator=%s state=%s", ErrElevatorNotRunning, e.id, e.operationState)
	}
	if e.currentFloor.Equals(f) {
		e.openDoorOnArrival()
		return nil
	}
	e.stopSchedule.Add(f)
	return nil
}

// 状態遷移は優先順に評価する:
//  1. running 以外なら何もしない
//  2. 開扉中: dwell が残っていれば消費して扉開のまま待機。
//     dwell が尽きていれば閉扉（schedule 空なら idle へ）。
//  3. schedule 空: 自動帰還オン & 非ホーム階なら home を積み直してフォールスルー、
//     それ以外は idle にする
//  4. 次目的階へ 1 階移動。到着時は schedule から削除し開扉（dwell リセット）。
func (e *Elevator) AdvanceOneTick() {
	if e.operationState != OperationStateRunning {
		return
	}
	if e.holdOpen {
		// 「開」ボタン保持中は move も自動閉扉もしない（扉開のままその場で待機）。
		e.doorState = DoorStateOpen
		return
	}
	if e.doorState == DoorStateOpen {
		if e.doorDwell > 0 {
			// 「扉開きの tick → dwell tick → 閉扉 tick」の中間。扉は開いたまま、
			// schedule や direction には触れない。
			e.doorDwell--
			return
		}
		e.doorState = DoorStateClosed
		// 出発方向のコミットが残ったまま閉扉すると、同階で逆方向の hall call を
		// 受けたとき「方向不整合のまま即時 serve しない」状態に陥り call が詰まる。
		// 次に行く階が無いなら閉扉と同 tick で idle に戻し、コミットを解除する。
		if e.stopSchedule.IsEmpty() && !e.shouldAutoReturn() {
			e.direction = DirectionIdle
		}
		return
	}
	if e.stopSchedule.IsEmpty() {
		if e.shouldAutoReturn() {
			// 空き状態でホーム階に居ない時は home を schedule に積み直して通常の
			// 移動ルートに合流させる。次の if 以降で 1 階分動く。
			e.stopSchedule.Add(e.homeFloor)
		} else {
			e.direction = DirectionIdle
			return
		}
	}
	target, ok := e.stopSchedule.NextFloor(e.currentFloor, e.direction)
	if !ok {
		e.direction = DirectionIdle
		return
	}
	if e.currentFloor.Equals(target) {
		// 防御的: AddDestination(current) は schedule に積まないので通常は到達しない。
		e.stopSchedule.Remove(e.currentFloor)
		e.openDoorOnArrival()
		return
	}
	if target.Value() > e.currentFloor.Value() {
		e.direction = DirectionUp
		e.currentFloor = e.currentFloor.Above()
	} else {
		e.direction = DirectionDown
		e.currentFloor = e.currentFloor.Below()
	}
	if e.currentFloor.Equals(target) {
		e.stopSchedule.Remove(e.currentFloor)
		e.openDoorOnArrival()
	}
}

func (e *Elevator) shouldAutoReturn() bool {
	return e.autoReturnEnabled && !e.currentFloor.Equals(e.homeFloor)
}

func (e *Elevator) VisibleStatusFrom(viewer Floor) VisibleElevatorStatus {
	if e.operationState != OperationStateRunning {
		return VisibleStatusUnavailable
	}
	if e.currentFloor.Equals(viewer) && e.doorState == DoorStateOpen {
		return VisibleStatusArrived
	}
	if e.direction == DirectionUp && e.currentFloor.Value() < viewer.Value() {
		return VisibleStatusApproaching
	}
	if e.direction == DirectionDown && e.currentFloor.Value() > viewer.Value() {
		return VisibleStatusApproaching
	}
	if e.currentFloor.Equals(viewer) {
		return VisibleStatusPassing
	}
	return VisibleStatusAway
}
