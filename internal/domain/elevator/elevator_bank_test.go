package elevator

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// fixedClock returns a deterministic monotonic time for tests.
func fixedClock(seq *int) time.Time {
	*seq++
	return time.Unix(int64(*seq), 0)
}

func mkBank(t *testing.T, min, max int, elevators ...struct {
	ID    string
	Floor int
}) *ElevatorBank {
	t.Helper()
	spec, err := NewBuildingSpec(NewFloor(min), NewFloor(max))
	if err != nil {
		t.Fatalf("NewBuildingSpec: %v", err)
	}
	bank := NewElevatorBank(spec, nil)
	for _, e := range elevators {
		eid, err := NewElevatorID(e.ID)
		if err != nil {
			t.Fatalf("NewElevatorID: %v", err)
		}
		if _, err := bank.AddElevator(eid, NewFloor(e.Floor)); err != nil {
			t.Fatalf("AddElevator: %v", err)
		}
	}
	return bank
}

type elevSpec = struct {
	ID    string
	Floor int
}

func nextHallCallID(seq *int) HallCallID {
	*seq++
	id, _ := NewHallCallID("call-" + strconv.Itoa(*seq))
	return id
}

func TestBank_AddElevator(t *testing.T) {
	cases := []struct {
		name    string
		seed    []elevSpec
		addID   string
		addAt   int
		wantErr error
	}{
		{name: "valid", addID: "ev-1", addAt: 1, wantErr: nil},
		{name: "out of range floor below", addID: "ev-1", addAt: 0, wantErr: ErrInvalidFloor},
		{name: "out of range floor above", addID: "ev-1", addAt: 11, wantErr: ErrInvalidFloor},
		{
			name:  "duplicate id",
			seed:  []elevSpec{{"ev-1", 1}},
			addID: "ev-1", addAt: 2, wantErr: errors.New("duplicate"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bank := mkBank(t, 1, 10, c.seed...)
			eid, _ := NewElevatorID(c.addID)
			_, err := bank.AddElevator(eid, NewFloor(c.addAt))
			if c.wantErr == nil {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			// 既知エラー型の場合は errors.Is で確認、それ以外は err != nil で十分。
			if errors.Is(c.wantErr, ErrInvalidFloor) {
				if !errors.Is(err, ErrInvalidFloor) {
					t.Errorf("err = %v, want ErrInvalidFloor", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

// PressHallButton の input -> 期待される結果を一気に表で検証する。
// 単一号機の bank を前提とした単純系のみを集約。複数号機の振る舞いは別テスト。
func TestBank_PressHallButton(t *testing.T) {
	type want struct {
		Created  bool
		Status   HallCallStatus
		Assigned string // "" = nil
	}
	cases := []struct {
		name      string
		seed      []elevSpec
		floor     int
		direction Direction
		wantWant  *want // nil なら wantErr のみ確認
		wantErr   error
	}{
		{
			name:  "happy path 1F -> 5F",
			seed:  []elevSpec{{"ev-1", 1}},
			floor: 5, direction: DirectionUp,
			wantWant: &want{Created: true, Status: HallCallStatusAssigned, Assigned: "ev-1"},
		},
		{
			// 既に呼び階に居る場合、AddDestination(currentFloor) は即時開扉するため
			// dispatch 直後に served までもっていく必要がある。
			name:  "already at floor -> served immediately",
			seed:  []elevSpec{{"ev-1", 5}},
			floor: 5, direction: DirectionUp,
			wantWant: &want{Created: true, Status: HallCallStatusServed, Assigned: "ev-1"},
		},
		{
			name:  "top floor up rejected",
			seed:  []elevSpec{{"ev-1", 1}},
			floor: 10, direction: DirectionUp,
			wantErr: ErrInvalidHallCallDirection,
		},
		{
			name:  "bottom floor down rejected",
			seed:  []elevSpec{{"ev-1", 1}},
			floor: 1, direction: DirectionDown,
			wantErr: ErrInvalidHallCallDirection,
		},
		{
			name:  "out of range",
			seed:  []elevSpec{{"ev-1", 1}},
			floor: 20, direction: DirectionUp,
			wantErr: ErrInvalidFloor,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bank := mkBank(t, 1, 10, c.seed...)
			cid, clk := 0, 0
			call, created, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(c.floor), c.direction, fixedClock(&clk))
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := want{Created: created, Status: call.Status()}
			if id := call.AssignedElevatorID(); id != nil {
				got.Assigned = id.String()
			}
			if diff := cmp.Diff(*c.wantWant, got); diff != "" {
				t.Errorf("hall call result mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBank_PressHallButton_IsIdempotentForActiveCall(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
	cid, clock := 0, 0
	first, _, _ := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock))
	second, created, _ := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock))
	if created {
		t.Errorf("expected created=false on duplicate")
	}
	if diff := cmp.Diff(first.ID(), second.ID()); diff != "" {
		t.Errorf("idempotent return mismatch (-want +got):\n%s", diff)
	}
	if got, want := len(bank.HallCalls()), 1; got != want {
		t.Errorf("len(HallCalls()) = %d want %d", got, want)
	}
}

func TestBank_PressHallButton_NoSideEffectsWhenAllStopped(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1}, elevSpec{"ev-2", 10})
	for _, id := range []string{"ev-1", "ev-2"} {
		ev, _ := bank.Elevator(ElevatorID(id))
		ev.operationState = OperationStateStopped
	}
	cid, clock := 0, 0
	_, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock))
	if !errors.Is(err, ErrNoAvailableElevator) {
		t.Errorf("expected ErrNoAvailableElevator, got %v", err)
	}
	if got := len(bank.HallCalls()); got != 0 {
		t.Errorf("hall call must not be registered on dispatch failure, got %d", got)
	}
}

func TestBank_PressCarButton(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, b *ElevatorBank) // bank 状態の初期化
		evID    string
		dest    int
		wantErr error
		// dest=current に対する開扉を確認するためのケース
		wantDoorOpenAtCurrent bool
	}{
		{
			name: "normal",
			evID: "ev-1", dest: 5,
		},
		{
			name: "out of range",
			evID: "ev-1", dest: 99,
			wantErr: ErrInvalidDestinationFloor,
		},
		{
			name: "elevator not found",
			evID: "nobody", dest: 5,
			wantErr: ErrElevatorNotFound,
		},
		{
			name: "not running",
			setup: func(t *testing.T, b *ElevatorBank) {
				ev, _ := b.Elevator("ev-1")
				ev.operationState = OperationStateMaintenance
			},
			evID: "ev-1", dest: 5,
			wantErr: ErrElevatorNotRunning,
		},
		{
			name: "same floor opens door",
			evID: "ev-1", dest: 1,
			wantDoorOpenAtCurrent: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
			if c.setup != nil {
				c.setup(t, bank)
			}
			err := bank.PressCarButton(ElevatorID(c.evID), NewFloor(c.dest))
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			ev, _ := bank.Elevator(ElevatorID(c.evID))
			if c.wantDoorOpenAtCurrent {
				if diff := cmp.Diff(DoorStateOpen, ev.DoorState()); diff != "" {
					t.Errorf("doorState mismatch (-want +got):\n%s", diff)
				}
				return
			}
			if !ev.HasDestination(NewFloor(c.dest)) {
				t.Errorf("schedule should contain %d", c.dest)
			}
		})
	}
}

func TestBank_AdvanceOneTick_MarksServedDoesNotDoubleTransition(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
	cid, clock := 0, 0
	call, _, _ := bank.PressHallButton(nextHallCallID(&cid), NewFloor(2), DirectionUp, fixedClock(&clock))

	// 1 tick で 1->2 到着 + door open + served までいく。
	bank.AdvanceOneTick()
	if call.Status() != HallCallStatusServed {
		t.Errorf("status = %s want served", call.Status())
	}
	// その後の tick で served から戻ってはいけない。
	bank.AdvanceOneTick()
	bank.AdvanceOneTick()
	if diff := cmp.Diff(HallCallStatusServed, call.Status()); diff != "" {
		t.Errorf("status mismatch (-want +got):\n%s", diff)
	}
}

func TestBank_PatchElevator(t *testing.T) {
	cases := []struct {
		name        string
		patchID     string
		buildPatch  func() ElevatorPatch
		wantErr     error
		wantOpState OperationState // wantErr==nil のときに確認
		wantFloor   int            // 0 なら確認しない
	}{
		{
			name:    "set operation stopped",
			patchID: "ev-1",
			buildPatch: func() ElevatorPatch {
				v := OperationStateStopped
				return ElevatorPatch{OperationState: &v}
			},
			wantOpState: OperationStateStopped,
		},
		{
			name:    "out of range floor rejected",
			patchID: "ev-1",
			buildPatch: func() ElevatorPatch {
				f := NewFloor(99)
				return ElevatorPatch{CurrentFloor: &f}
			},
			wantErr: ErrInvalidFloor,
		},
		{
			name:    "elevator not found",
			patchID: "nope",
			buildPatch: func() ElevatorPatch {
				v := OperationStateStopped
				return ElevatorPatch{OperationState: &v}
			},
			wantErr: ErrElevatorNotFound,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
			_, err := bank.PatchElevator(ElevatorID(c.patchID), c.buildPatch())
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
				// validation error 時は他フィールドも書き換わってはいけない（部分適用なし）
				ev, _ := bank.Elevator("ev-1")
				if ev != nil && ev.CurrentFloor().Value() != 1 {
					t.Errorf("currentFloor leaked: %d", ev.CurrentFloor().Value())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			ev, _ := bank.Elevator("ev-1")
			if diff := cmp.Diff(c.wantOpState, ev.OperationState()); diff != "" {
				t.Errorf("operationState mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBank_PatchElevator_StoppedDoesNotMove(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
	stopped := OperationStateStopped
	if _, err := bank.PatchElevator("ev-1", ElevatorPatch{OperationState: &stopped}); err != nil {
		t.Fatalf("PatchElevator: %v", err)
	}
	ev, _ := bank.Elevator("ev-1")
	ev.stopSchedule.Add(NewFloor(5))
	bank.AdvanceOneTick()
	if got := ev.CurrentFloor().Value(); got != 1 {
		t.Errorf("stopped elevator moved: %d", got)
	}
}

func TestBank_CancelHallCall(t *testing.T) {
	cases := []struct {
		name       string
		seedCall   bool   // hall call を事前に登録するか
		cancelID   string // 空なら登録した call の ID を使う
		wantErr    error
		wantStatus HallCallStatus
	}{
		{
			name:       "cancel registered call",
			seedCall:   true,
			wantStatus: HallCallStatusCanceled,
		},
		{
			name:     "missing id returns ErrHallCallNotFound",
			cancelID: "nope",
			wantErr:  ErrHallCallNotFound,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
			cid, clock := 0, 0
			var call *HallCall
			if c.seedCall {
				out, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock))
				if err != nil {
					t.Fatalf("PressHallButton: %v", err)
				}
				call = out
			}
			cancelID := c.cancelID
			if cancelID == "" {
				cancelID = string(call.ID())
			}
			err := bank.CancelHallCall(HallCallID(cancelID))
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(c.wantStatus, call.Status()); diff != "" {
				t.Errorf("status mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBank_DrainEvents_HallCallLifecycle(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
	cid, clock := 0, 0
	if _, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock)); err != nil {
		t.Fatalf("PressHallButton: %v", err)
	}
	events := bank.DrainEvents()
	if got := len(events); got != 1 {
		t.Fatalf("len(events) = %d want 1, got %#v", got, events)
	}
	if _, ok := events[0].(HallCallRequested); !ok {
		t.Errorf("event[0] = %T want HallCallRequested", events[0])
	}
	// drain 済みなので 2 回目は空
	if next := bank.DrainEvents(); len(next) != 0 {
		t.Errorf("len(events) after drain = %d want 0", len(next))
	}

	// tick で到着 + served の 2 イベント
	for range 4 {
		bank.AdvanceOneTick()
	}
	events = bank.DrainEvents()
	got := struct{ Arrived, Served bool }{}
	for _, e := range events {
		if _, ok := e.(ElevatorArrived); ok {
			got.Arrived = true
		}
		if _, ok := e.(HallCallServed); ok {
			got.Served = true
		}
	}
	want := struct{ Arrived, Served bool }{Arrived: true, Served: true}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("expected Arrived + Served events, mismatch (-want +got):\n%s\nevents=%#v", diff, events)
	}
}

func TestBank_DrainEvents_StateChange(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
	bank.DrainEvents() // 初期化のノイズを除外

	stopped := OperationStateStopped
	if _, err := bank.PatchElevator("ev-1", ElevatorPatch{OperationState: &stopped}); err != nil {
		t.Fatalf("PatchElevator: %v", err)
	}
	events := bank.DrainEvents()
	if got := len(events); got != 1 {
		t.Fatalf("len(events) = %d want 1", got)
	}
	ev, ok := events[0].(ElevatorStateChanged)
	if !ok {
		t.Fatalf("event[0] = %T want ElevatorStateChanged", events[0])
	}
	want := struct{ From, To OperationState }{From: OperationStateRunning, To: OperationStateStopped}
	got := struct{ From, To OperationState }{From: ev.From, To: ev.To}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("state change mismatch (-want +got):\n%s", diff)
	}

	// 同じ state を再 set しても event は出ない
	if _, err := bank.PatchElevator("ev-1", ElevatorPatch{OperationState: &stopped}); err != nil {
		t.Fatalf("PatchElevator: %v", err)
	}
	if next := bank.DrainEvents(); len(next) != 0 {
		t.Errorf("no-op patch should not emit, got %#v", next)
	}
}

func TestBank_VisibleElevatorsFrom_RejectsOutOfRange(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
	if _, err := bank.VisibleElevatorsFrom(NewFloor(11)); !errors.Is(err, ErrInvalidFloor) {
		t.Errorf("expected ErrInvalidFloor, got %v", err)
	}
}

// Scenario A from docs/test-cases.md §6:
//
//	building 1-10, ev-1 at 1F
//	1. PressHallButton(5, up)  → assigned to ev-1, schedule={5}
//	2. tick × 4                → ev-1 at 5F, door open, hall call served
//	3. PressCarButton(ev-1, 8) → schedule={8}
//	4. tick × 2                → dwell 消費 → door=closed
//	5. tick × 3                → ev-1 at 8F, door open
func TestBank_ScenarioA_SingleElevatorRoundTrip(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1})
	cid, clock := 0, 0
	call, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock))
	if err != nil {
		t.Fatalf("PressHallButton: %v", err)
	}
	for range 4 {
		bank.AdvanceOneTick()
	}
	ev, _ := bank.Elevator(ElevatorID("ev-1"))
	type ck struct {
		Floor  int
		Door   DoorState
		Status HallCallStatus
	}
	want := ck{Floor: 5, Door: DoorStateOpen, Status: HallCallStatusServed}
	got := ck{Floor: ev.CurrentFloor().Value(), Door: ev.DoorState(), Status: call.Status()}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("after 4 ticks mismatch (-want +got):\n%s", diff)
	}

	if err := bank.PressCarButton(ElevatorID("ev-1"), NewFloor(8)); err != nil {
		t.Fatalf("PressCarButton: %v", err)
	}
	// dwell tick → 閉扉 tick の 2 段階で扉が閉まる。
	bank.AdvanceOneTick()
	if ev.DoorState() != DoorStateOpen {
		t.Errorf("after dwell tick: door=%s want open", ev.DoorState())
	}
	bank.AdvanceOneTick()
	if ev.DoorState() != DoorStateClosed {
		t.Errorf("after close tick: door=%s want closed", ev.DoorState())
	}
	for range 3 {
		bank.AdvanceOneTick()
	}
	want2 := struct {
		Floor int
		Door  DoorState
	}{Floor: 8, Door: DoorStateOpen}
	got2 := struct {
		Floor int
		Door  DoorState
	}{Floor: ev.CurrentFloor().Value(), Door: ev.DoorState()}
	if diff := cmp.Diff(want2, got2); diff != "" {
		t.Errorf("after 3 more ticks mismatch (-want +got):\n%s", diff)
	}
}

// 同階で up/down を続けて押すと、扉開きで up にコミット中の号機は下呼びの候補から
// 外れ、別号機にアサインされる。「両ボタン同時消滅」の元凶を塞ぐ核となる挙動。
func TestBank_PressHallButton_SameFloorOppositeDirectionRoutesToOtherElevator(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 5}, elevSpec{"ev-2", 1})
	cid, clock := 0, 0

	upCall, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock))
	if err != nil {
		t.Fatalf("PressHallButton(5,up): %v", err)
	}
	type want struct {
		Assigned string
		Status   HallCallStatus
	}
	gotUp := want{Assigned: upCall.AssignedElevatorID().String(), Status: upCall.Status()}
	wantUp := want{Assigned: "ev-1", Status: HallCallStatusServed}
	if diff := cmp.Diff(wantUp, gotUp); diff != "" {
		t.Fatalf("up call mismatch (-want +got):\n%s", diff)
	}
	ev1, _ := bank.Elevator(ElevatorID("ev-1"))
	if ev1.Direction() != DirectionUp {
		t.Errorf("ev-1 direction=%s want up（idle 同階受け入れ時に方向確定するはず）", ev1.Direction())
	}

	downCall, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionDown, fixedClock(&clock))
	if err != nil {
		t.Fatalf("PressHallButton(5,down): %v", err)
	}
	if got := downCall.AssignedElevatorID(); got == nil || got.String() != "ev-2" {
		t.Errorf("down: assigned=%v want ev-2（ev-1 は up コミット中なので除外）", got)
	}
	if downCall.Status() == HallCallStatusServed {
		t.Errorf("down: ev-2 はまだ到着してないので served にならないはず")
	}
}

// 号機が階に到着しただけでは、その階の逆方向 hall call は消えない（方向ガード）。
func TestBank_AdvanceOneTick_DoesNotServeOppositeDirectionAtSameFloor(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1}, elevSpec{"ev-2", 10})
	cid, clock := 0, 0

	upCall, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock))
	if err != nil {
		t.Fatalf("PressHallButton(5,up): %v", err)
	}
	downCall, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionDown, fixedClock(&clock))
	if err != nil {
		t.Fatalf("PressHallButton(5,down): %v", err)
	}
	if upCall.AssignedElevatorID().String() == downCall.AssignedElevatorID().String() {
		t.Fatalf("up と down が同号機にアサインされた: %s", upCall.AssignedElevatorID())
	}

	// ev-1 が 5F に到着するまで進める（4 tick）。同 tick で ev-2 も 6F まで降りているはず。
	for range 4 {
		bank.AdvanceOneTick()
	}
	type ck struct{ Up, Down HallCallStatus }
	want := ck{Up: HallCallStatusServed, Down: HallCallStatusAssigned}
	got := ck{Up: upCall.Status(), Down: downCall.Status()}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("status mismatch (-want +got):\n%s", diff)
	}
}

// 閉扉と同時に schedule 空なら direction=idle に戻る。コミットが残らないので
// 同階逆方向呼びを 1 tick 後に再受け入れできる（1 号機運用の救済）。
func TestBank_AdvanceOneTick_ResetsDirectionOnDoorCloseWhenIdle(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 5})
	cid, clock := 0, 0

	if _, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionUp, fixedClock(&clock)); err != nil {
		t.Fatalf("PressHallButton(5,up): %v", err)
	}
	ev, _ := bank.Elevator(ElevatorID("ev-1"))
	if ev.Direction() != DirectionUp {
		t.Fatalf("setup: direction=%s want up", ev.Direction())
	}
	bank.AdvanceOneTick() // dwell 消費（扉開きのまま）
	bank.AdvanceOneTick() // 閉扉 + schedule 空 → idle
	if ev.Direction() != DirectionIdle {
		t.Errorf("direction=%s want idle（閉扉と同 tick で idle 戻し）", ev.Direction())
	}

	// idle に戻ったので同階の down 呼びを取れる。
	downCall, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(5), DirectionDown, fixedClock(&clock))
	if err != nil {
		t.Fatalf("PressHallButton(5,down) after idle reset: %v", err)
	}
	type ck struct {
		Status    HallCallStatus
		Direction Direction
	}
	want := ck{Status: HallCallStatusServed, Direction: DirectionDown}
	got := ck{Status: downCall.Status(), Direction: ev.Direction()}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("after re-pickup mismatch (-want +got):\n%s", diff)
	}
}

// 単独号機の救済シナリオを表で。1 tick 上限を 15 ticks にして全 served 化を待つ。
func TestBank_SingleElevator_HandlesMixedCalls(t *testing.T) {
	type call struct {
		floor     int
		direction Direction
	}
	cases := []struct {
		name  string
		seed  []elevSpec
		calls []call
	}{
		{
			name:  "1 elevator below, down call from 5F",
			seed:  []elevSpec{{"ev-1", 1}},
			calls: []call{{5, DirectionDown}},
		},
		{
			name:  "1 elevator must serve up@3 and down@5",
			seed:  []elevSpec{{"ev-1", 1}},
			calls: []call{{3, DirectionUp}, {5, DirectionDown}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bank := mkBank(t, 1, 10, c.seed...)
			cid, clock := 0, 0
			placed := make([]*HallCall, 0, len(c.calls))
			for _, cl := range c.calls {
				h, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(cl.floor), cl.direction, fixedClock(&clock))
				if err != nil {
					t.Fatalf("PressHallButton(%d,%s): %v", cl.floor, cl.direction, err)
				}
				placed = append(placed, h)
			}
			for i := 0; i < 20 && !allServed(placed); i++ {
				bank.AdvanceOneTick()
			}
			gotStatuses := make([]HallCallStatus, len(placed))
			wantStatuses := make([]HallCallStatus, len(placed))
			for i, h := range placed {
				gotStatuses[i] = h.Status()
				wantStatuses[i] = HallCallStatusServed
			}
			if diff := cmp.Diff(wantStatuses, gotStatuses); diff != "" {
				t.Errorf("statuses mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func allServed(calls []*HallCall) bool {
	for _, c := range calls {
		if c.Status() != HallCallStatusServed {
			return false
		}
	}
	return true
}

// Scenario B: dispatch routes calls to the correct elevator and both end up served.
func TestBank_ScenarioB_DispatchRouting(t *testing.T) {
	bank := mkBank(t, 1, 10, elevSpec{"ev-1", 1}, elevSpec{"ev-2", 10})
	cid, clock := 0, 0

	callDown, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(8), DirectionDown, fixedClock(&clock))
	if err != nil {
		t.Fatalf("PressHallButton(8,down): %v", err)
	}
	callUp, _, err := bank.PressHallButton(nextHallCallID(&cid), NewFloor(2), DirectionUp, fixedClock(&clock))
	if err != nil {
		t.Fatalf("PressHallButton(2,up): %v", err)
	}
	type ck struct{ Down, Up string }
	wantAssigned := ck{Down: "ev-2", Up: "ev-1"}
	gotAssigned := ck{Down: callDown.AssignedElevatorID().String(), Up: callUp.AssignedElevatorID().String()}
	if diff := cmp.Diff(wantAssigned, gotAssigned); diff != "" {
		t.Errorf("dispatch mismatch (-want +got):\n%s", diff)
	}

	for i := 0; i < 10 && !allServed([]*HallCall{callDown, callUp}); i++ {
		bank.AdvanceOneTick()
	}
	type sk struct{ Down, Up HallCallStatus }
	wantStatus := sk{Down: HallCallStatusServed, Up: HallCallStatusServed}
	gotStatus := sk{Down: callDown.Status(), Up: callUp.Status()}
	if diff := cmp.Diff(wantStatus, gotStatus); diff != "" {
		t.Errorf("status mismatch (-want +got):\n%s", diff)
	}
}
