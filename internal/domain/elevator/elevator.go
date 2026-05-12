package elevator

import "fmt"

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
}

func NewElevator(id ElevatorID, initial Floor) *Elevator {
	return &Elevator{
		id:             id,
		currentFloor:   initial,
		direction:      DirectionIdle,
		doorState:      DoorStateClosed,
		operationState: OperationStateRunning,
		stopSchedule:   NewStopSchedule(),
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
func (e *Elevator) OpenDoor() {
	e.doorState = DoorStateOpen
	e.holdOpen = true
}

// 閉ボタン: hold-open を解除し扉を閉じる。次 tick から通常の運行に戻る。
func (e *Elevator) CloseDoor() {
	e.doorState = DoorStateClosed
	e.holdOpen = false
}

func (e *Elevator) HoldOpen() bool { return e.holdOpen }

// 行先 == 現在階のときは schedule に積まずに即座に開扉する。
// 範囲検証は集約 (ElevatorBank) 側で行う前提。
func (e *Elevator) AddDestination(f Floor) error {
	if !e.IsRunning() {
		return fmt.Errorf("%w: elevator=%s state=%s", ErrElevatorNotRunning, e.id, e.operationState)
	}
	if e.currentFloor.Equals(f) {
		e.doorState = DoorStateOpen
		return nil
	}
	e.stopSchedule.Add(f)
	return nil
}

// 状態遷移は優先順に評価する:
//  1. running 以外なら何もしない
//  2. 開扉中なら閉扉のみ（移動しない）。schedule が空ならその tick で idle に戻す
//  3. schedule 空なら idle にする
//  4. それ以外は次目的階へ 1 階移動。到着時は schedule から削除し開扉。
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
		e.doorState = DoorStateClosed
		// 出発方向のコミットが残ったまま閉扉すると、同階で逆方向の hall call を
		// 受けたとき「方向不整合のまま即時 serve しない」状態に陥り call が詰まる。
		// 次に行く階が無いなら閉扉と同 tick で idle に戻し、コミットを解除する。
		if e.stopSchedule.IsEmpty() {
			e.direction = DirectionIdle
		}
		return
	}
	if e.stopSchedule.IsEmpty() {
		e.direction = DirectionIdle
		return
	}
	target, ok := e.stopSchedule.NextFloor(e.currentFloor, e.direction)
	if !ok {
		e.direction = DirectionIdle
		return
	}
	if e.currentFloor.Equals(target) {
		// 防御的: AddDestination(current) は schedule に積まないので通常は到達しない。
		e.stopSchedule.Remove(e.currentFloor)
		e.doorState = DoorStateOpen
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
		e.doorState = DoorStateOpen
	}
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
