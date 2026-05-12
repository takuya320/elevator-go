package elevator

import (
	"fmt"
	"sort"
	"time"
)

// 集約全体の不変条件:
//   - 階・方向は BuildingSpec を満たす
//   - (floor, direction) ごとに active な HallCall は最大 1 つ
//   - HallCall は running な elevator にのみ割り当てる。失敗時は登録自体を行わない
//   - assignedElevatorID は当バンクに存在する elevator を指す
//
// 並行不可: usecase.Locker による直列化が前提。
type ElevatorBank struct {
	spec      BuildingSpec
	elevators map[ElevatorID]*Elevator
	hallCalls map[HallCallID]*HallCall
	policy    DispatchPolicy
	// drain されるまで集約内に溜まる。タイムスタンプは付かない（usecase 層で付与）。
	events []DomainEvent
}

func (b *ElevatorBank) emit(e DomainEvent) {
	b.events = append(b.events, e)
}

// DrainEvents は呼び出しごとに溜まっていたイベントを返し、内部バッファを空にする。
// 同じイベントは二度返さない。
func (b *ElevatorBank) DrainEvents() []DomainEvent {
	if len(b.events) == 0 {
		return nil
	}
	out := b.events
	b.events = nil
	return out
}

// policy が nil のときは NearestAvailableElevatorPolicy を採用する（通常運用のデフォルト）。
func NewElevatorBank(spec BuildingSpec, policy DispatchPolicy) *ElevatorBank {
	if policy == nil {
		policy = NewNearestAvailableElevatorPolicy()
	}
	return &ElevatorBank{
		spec:      spec,
		elevators: map[ElevatorID]*Elevator{},
		hallCalls: map[HallCallID]*HallCall{},
		policy:    policy,
	}
}

func (b *ElevatorBank) Spec() BuildingSpec { return b.spec }

func (b *ElevatorBank) AddElevator(id ElevatorID, initial Floor) (*Elevator, error) {
	if !b.spec.Contains(initial) {
		return nil, fmt.Errorf("%w: initial floor %d outside [%d, %d]",
			ErrInvalidFloor, initial.Value(), b.spec.Min().Value(), b.spec.Max().Value())
	}
	if _, exists := b.elevators[id]; exists {
		return nil, fmt.Errorf("elevator %s already exists", id)
	}
	e := NewElevator(id, initial)
	b.elevators[id] = e
	return e, nil
}

func (b *ElevatorBank) Elevator(id ElevatorID) (*Elevator, bool) {
	e, ok := b.elevators[id]
	return e, ok
}

// ID 昇順で返す（ログ・テストで順序が安定する）。
func (b *ElevatorBank) Elevators() []*Elevator {
	return b.sortedElevators()
}

// ID 昇順で返す（ログ・テストで順序が安定する）。
func (b *ElevatorBank) HallCalls() []*HallCall {
	out := make([]*HallCall, 0, len(b.hallCalls))
	for _, c := range b.hallCalls {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// 動作:
//   - active な (floor, direction) があれば既存を created=false で返す（冪等）
//   - 配車失敗時は call を登録しない（無副作用）
//   - 成功時のみ assigned 状態で登録、対象号機の schedule に追加し created=true
//
// id, createdAt はドメインを ID 生成・clock から切り離すため呼び出し側から渡す。
func (b *ElevatorBank) PressHallButton(
	id HallCallID,
	floor Floor,
	direction Direction,
	createdAt time.Time,
) (call *HallCall, created bool, err error) {
	if !b.spec.Contains(floor) {
		return nil, false, fmt.Errorf("%w: floor=%d outside [%d, %d]",
			ErrInvalidFloor, floor.Value(), b.spec.Min().Value(), b.spec.Max().Value())
	}
	if !b.spec.CanCall(floor, direction) {
		return nil, false, fmt.Errorf("%w: floor=%d direction=%s",
			ErrInvalidHallCallDirection, floor.Value(), direction)
	}

	if existing := b.findActiveHallCall(floor, direction); existing != nil {
		return existing, false, nil
	}

	newCall, err := NewHallCall(id, floor, direction, createdAt)
	if err != nil {
		return nil, false, err
	}
	candidate, err := b.policy.SelectElevator(newCall, b.dispatchCandidates(floor, direction))
	if err != nil {
		return nil, false, err
	}
	if err := newCall.AssignTo(candidate.ID()); err != nil {
		return nil, false, err
	}
	if err := candidate.AddDestination(floor); err != nil {
		// 防御的: SelectElevator は running しか返さないので通常は失敗しない。
		return nil, false, err
	}
	// 同階指定で AddDestination が即時開扉した場合、次の tick で扉が閉まる前に
	// served にしないと AdvanceOneTick の serve 判定が取りこぼし、call が永久に
	// assigned のまま残る。
	// idle 号機が同階呼びを受けたときは、出発方向を call の方向に確定する
	// （以後同階の逆方向呼びは dispatchCandidates で除外できるようにするため）。
	// 方向不整合の候補は dispatchCandidates ですでに除外しているため、ここでは
	// 方向ガードを重ねず、コミットだけ行う。
	served := false
	if candidate.IsAtFloorWithDoorOpen(floor) {
		if candidate.Direction() == DirectionIdle {
			candidate.direction = direction
		}
		newCall.MarkServed()
		served = true
	}
	b.hallCalls[id] = newCall
	b.emit(HallCallRequested{CallID: id, Floor: floor, Direction: direction, ElevatorID: candidate.ID()})
	if served {
		b.emit(HallCallServed{CallID: id, Floor: floor, ElevatorID: candidate.ID()})
	}
	return newCall, true, nil
}

// 同階で逆方向の呼びを既に背負っている号機を候補から外す。「上ボタンで開いた
// 号機が続けて下ボタンも飲み込む」のを防ぐ核となるフィルタ。除外条件は 2 つ:
//
//  1. その号機に同階の逆方向 active call が既にアサインされている
//     （未到着時の重複アサイン防止）
//  2. 扉開きで逆方向にコミット中（idle 同階受け入れ後の direction で判定）
//
// 候補が空になった場合は ErrNoAvailableElevator を policy が返す（1 号機運用で
// 同時押しした場合の 2 回目は 409 になるが、扉が閉じれば direction が idle に
// 戻り再度受け入れ可能になる）。
func (b *ElevatorBank) dispatchCandidates(floor Floor, direction Direction) []*Elevator {
	all := b.sortedElevators()
	out := make([]*Elevator, 0, len(all))
	for _, e := range all {
		if b.hasOppositeAssignment(e.ID(), floor, direction) {
			continue
		}
		if e.IsAtFloorWithDoorOpen(floor) && !canServe(e.Direction(), direction) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (b *ElevatorBank) hasOppositeAssignment(eid ElevatorID, floor Floor, direction Direction) bool {
	for _, c := range b.hallCalls {
		if !c.IsActive() || c.Direction() == direction || !c.Floor().Equals(floor) {
			continue
		}
		assigned := c.AssignedElevatorID()
		if assigned != nil && *assigned == eid {
			return true
		}
	}
	return false
}

// 号機の「現在の出発方向コミット」が呼び方向を取れるか。
// idle はまだ未コミット扱いとして両方向とも取れる。
func canServe(elevatorDir, callDir Direction) bool {
	return elevatorDir == DirectionIdle || elevatorDir == callDir
}

// 管理 API 用の部分更新。指定されたフィールドのみ更新する。
// 検証エラー時は何も書き換えない（部分適用なし）。
//
// 注意: 扉開き中の号機に Direction=idle を書き戻すと、dispatchCandidates の
// 「扉開きで逆方向にコミット中の号機を除外」フィルタが効かなくなり、同階の
// 逆方向 hall call を即時 serve してしまう。デバッグ目的の操作に限ること。
type ElevatorPatch struct {
	CurrentFloor   *Floor
	Direction      *Direction
	DoorState      *DoorState
	OperationState *OperationState
}

func (b *ElevatorBank) PatchElevator(id ElevatorID, p ElevatorPatch) (*Elevator, error) {
	e, ok := b.elevators[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrElevatorNotFound, id)
	}
	if p.CurrentFloor != nil && !b.spec.Contains(*p.CurrentFloor) {
		return nil, fmt.Errorf("%w: floor=%d outside [%d, %d]",
			ErrInvalidFloor, p.CurrentFloor.Value(), b.spec.Min().Value(), b.spec.Max().Value())
	}
	if p.Direction != nil && !p.Direction.IsValid() {
		return nil, fmt.Errorf("invalid direction: %s", *p.Direction)
	}
	prevState := e.OperationState()
	if p.CurrentFloor != nil {
		e.currentFloor = *p.CurrentFloor
	}
	if p.Direction != nil {
		e.direction = *p.Direction
	}
	if p.DoorState != nil {
		e.doorState = *p.DoorState
	}
	if p.OperationState != nil && *p.OperationState != prevState {
		e.operationState = *p.OperationState
		b.emit(ElevatorStateChanged{ElevatorID: id, From: prevState, To: *p.OperationState})
	}
	return e, nil
}

// 「開」ボタン: 扉を開けて hold-open 状態へ。
func (b *ElevatorBank) OpenDoor(eid ElevatorID) (*Elevator, error) {
	e, ok := b.elevators[eid]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrElevatorNotFound, eid)
	}
	e.OpenDoor()
	return e, nil
}

// 「閉」ボタン: hold-open を解除し扉を閉じる。
func (b *ElevatorBank) CloseDoor(eid ElevatorID) (*Elevator, error) {
	e, ok := b.elevators[eid]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrElevatorNotFound, eid)
	}
	e.CloseDoor()
	return e, nil
}

// 既存 hall call をキャンセル状態に遷移させる。
// schedule や elevator state には触れない（予定階に到着しても serve 判定は走らない）。
func (b *ElevatorBank) CancelHallCall(id HallCallID) error {
	c, ok := b.hallCalls[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrHallCallNotFound, id)
	}
	if c.Cancel() {
		b.emit(HallCallCanceled{CallID: id})
	}
	return nil
}

func (b *ElevatorBank) PressCarButton(eid ElevatorID, dest Floor) error {
	e, ok := b.elevators[eid]
	if !ok {
		return fmt.Errorf("%w: %s", ErrElevatorNotFound, eid)
	}
	if !b.spec.Contains(dest) {
		return fmt.Errorf("%w: floor=%d outside [%d, %d]",
			ErrInvalidDestinationFloor, dest.Value(), b.spec.Min().Value(), b.spec.Max().Value())
	}
	if err := e.AddDestination(dest); err != nil {
		return err
	}
	b.emit(CarCallRequested{ElevatorID: eid, Floor: dest})
	return nil
}

// 各号機を進めた後、open 中の階に対応する assigned な hall call を served にする。
// 進行と served 判定を別段階に分けることで多重遷移を避ける。
func (b *ElevatorBank) AdvanceOneTick() {
	// tick 内で扉が open に遷移した号機を arrived として記録。
	for _, e := range b.sortedElevators() {
		prevDoor := e.DoorState()
		e.AdvanceOneTick()
		if prevDoor != DoorStateOpen && e.DoorState() == DoorStateOpen {
			b.emit(ElevatorArrived{ElevatorID: e.ID(), Floor: e.CurrentFloor()})
		}
	}
	for _, c := range b.HallCalls() {
		if c.Status() != HallCallStatusAssigned {
			continue
		}
		eid := c.AssignedElevatorID()
		if eid == nil {
			continue
		}
		e, ok := b.elevators[*eid]
		if !ok {
			continue
		}
		// serve は方向を見ない（扉開きで同階に居れば消化）。同階の up/down 両方が
		// 同号機にアサインされる事態は dispatchCandidates で防いであるので、ここで
		// 重ねて方向ガードを掛けると、SCAN 配車で「上昇途中に下方向呼びの階を
		// 通過したとき serve できず詰まる」regression を生む。
		if e.IsAtFloorWithDoorOpen(c.Floor()) {
			c.MarkServed()
			b.emit(HallCallServed{CallID: c.ID(), Floor: c.Floor(), ElevatorID: *eid})
		}
	}
}

func (b *ElevatorBank) VisibleElevatorsFrom(viewer Floor) ([]VisibleElevator, error) {
	if !b.spec.Contains(viewer) {
		return nil, fmt.Errorf("%w: floor=%d outside [%d, %d]",
			ErrInvalidFloor, viewer.Value(), b.spec.Min().Value(), b.spec.Max().Value())
	}
	out := make([]VisibleElevator, 0, len(b.elevators))
	for _, e := range b.sortedElevators() {
		out = append(out, VisibleElevator{
			ElevatorID:     e.ID(),
			CurrentFloor:   e.CurrentFloor(),
			Direction:      e.Direction(),
			DoorState:      e.DoorState(),
			OperationState: e.OperationState(),
			VisibleStatus:  e.VisibleStatusFrom(viewer),
		})
	}
	return out, nil
}

func (b *ElevatorBank) findActiveHallCall(floor Floor, direction Direction) *HallCall {
	for _, c := range b.hallCalls {
		if c.IsActive() && c.Floor().Equals(floor) && c.Direction() == direction {
			return c
		}
	}
	return nil
}

func (b *ElevatorBank) sortedElevators() []*Elevator {
	out := make([]*Elevator, 0, len(b.elevators))
	for _, e := range b.elevators {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
