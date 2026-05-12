package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"elevator-go/internal/domain/elevator"
	"elevator-go/internal/infrastructure/persistence/memory"
)

// --- shared fakes ---------------------------------------------------------

type fakeLocker struct{ locks, unlocks int }

func (f *fakeLocker) Lock()   { f.locks++ }
func (f *fakeLocker) Unlock() { f.unlocks++ }

type fakeClock struct{ t time.Time }

func (f fakeClock) Now() time.Time { return f.t }

type fakeIDGenerator struct {
	prefix string
	seq    int
}

func (f *fakeIDGenerator) NewID() string {
	f.seq++
	return fmt.Sprintf("%s-%d", f.prefix, f.seq)
}

// --- shared setup helpers ------------------------------------------------

type fixtures struct {
	ctx    context.Context
	repo   *memory.ElevatorBankRepository
	simClk *memory.SimulationClock
	locker *fakeLocker
	clock  fakeClock
	idGen  *fakeIDGenerator
}

func setupDefault(t *testing.T) *fixtures {
	t.Helper()
	ctx := context.Background()
	repo := memory.NewElevatorBankRepository()
	simClk := memory.NewSimulationClock()
	locker := &fakeLocker{}
	clock := fakeClock{t: time.Unix(1700000000, 0).UTC()}
	idGen := &fakeIDGenerator{prefix: "call"}

	uc := NewResetSimulation(repo, simClk, locker, DefaultFloorRange, DefaultElevators)
	if _, err := uc.Execute(ctx, ResetSimulationInput{}); err != nil {
		t.Fatalf("seed reset: %v", err)
	}
	return &fixtures{ctx: ctx, repo: repo, simClk: simClk, locker: locker, clock: clock, idGen: idGen}
}

// --- tests ---------------------------------------------------------------

// PressHallButton input -> 期待値の表。HappyPath/Idempotency/エラーマッピングを集約。
func TestPressHallButton(t *testing.T) {
	type want struct {
		Created     bool
		Status      string
		HasAssigned bool
	}
	cases := []struct {
		name     string
		runs     []PressHallButtonInput // 同 fixture で順に実行
		wantLast want
		wantErr  bool
	}{
		{
			name:     "happy path creates assigned",
			runs:     []PressHallButtonInput{{Floor: 5, Direction: "up"}},
			wantLast: want{Created: true, Status: "assigned", HasAssigned: true},
		},
		{
			name: "second call is idempotent (same id, Created=false)",
			runs: []PressHallButtonInput{
				{Floor: 5, Direction: "up"},
				{Floor: 5, Direction: "up"},
			},
			wantLast: want{Created: false, Status: "assigned", HasAssigned: true},
		},
		{
			name:    "out-of-range floor returns error",
			runs:    []PressHallButtonInput{{Floor: 99, Direction: "up"}},
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupDefault(t)
			uc := NewPressHallButton(f.repo, f.locker, f.clock, f.idGen)
			var (
				out *PressHallButtonOutput
				err error
			)
			for _, in := range c.runs {
				out, err = uc.Execute(f.ctx, in)
			}
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got out=%+v", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			got := want{
				Created:     out.Created,
				Status:      out.Call.Status,
				HasAssigned: out.Call.AssignedElevatorID != nil,
			}
			if diff := cmp.Diff(c.wantLast, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPressHallButton_Idempotent_ReturnsSameID(t *testing.T) {
	f := setupDefault(t)
	uc := NewPressHallButton(f.repo, f.locker, f.clock, f.idGen)
	first, _ := uc.Execute(f.ctx, PressHallButtonInput{Floor: 5, Direction: "up"})
	second, _ := uc.Execute(f.ctx, PressHallButtonInput{Floor: 5, Direction: "up"})
	if diff := cmp.Diff(first.Call.ID, second.Call.ID); diff != "" {
		t.Errorf("ID mismatch on idempotent call (-first +second):\n%s", diff)
	}
}

func TestPressHallButton_HappyPath_CreatedAtFromClock(t *testing.T) {
	f := setupDefault(t)
	uc := NewPressHallButton(f.repo, f.locker, f.clock, f.idGen)
	out, err := uc.Execute(f.ctx, PressHallButtonInput{Floor: 5, Direction: "up"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.Call.CreatedAt.Equal(f.clock.t) {
		t.Errorf("CreatedAt = %v, want %v", out.Call.CreatedAt, f.clock.t)
	}
}

func TestPressHallButton_LockReleasedOnError(t *testing.T) {
	f := setupDefault(t)
	uc := NewPressHallButton(f.repo, f.locker, f.clock, f.idGen)

	prev := struct{ Locks, Unlocks int }{f.locker.locks, f.locker.unlocks}
	_, err := uc.Execute(f.ctx, PressHallButtonInput{Floor: 99, Direction: "up"})
	if err == nil {
		t.Fatalf("expected error for out-of-range floor")
	}
	got := struct{ Locks, Unlocks int }{f.locker.locks - prev.Locks, f.locker.unlocks - prev.Unlocks}
	want := struct{ Locks, Unlocks int }{1, 1}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("lock/unlock balance mismatch (-want +got):\n%s", diff)
	}
}

func TestPressCarButton(t *testing.T) {
	f := setupDefault(t)
	uc := NewPressCarButton(f.repo, f.locker)

	if err := uc.Execute(f.ctx, PressCarButtonInput{ElevatorID: "ev-1", DestinationFloor: 7}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	bank, _ := f.repo.Find(f.ctx)
	ev, _ := bank.Elevator("ev-1")
	if !ev.HasDestination(elevator.NewFloor(7)) {
		t.Errorf("ev-1 schedule should contain 7")
	}
}

// AdvanceTick の連続実行で Tick が単調増加し、各号機の floor range が正しく載ること。
func TestAdvanceTick_AdvancesAndExposesFloorRange(t *testing.T) {
	f := setupDefault(t)
	advance := NewAdvanceTick(f.repo, f.simClk, f.locker, f.clock)

	cases := []struct {
		name     string
		wantTick int
	}{
		{"first tick", 1},
		{"second tick", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := advance.Execute(f.ctx)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if diff := cmp.Diff(c.wantTick, out.Tick); diff != "" {
				t.Errorf("Tick mismatch (-want +got):\n%s", diff)
			}
			if got := len(out.Elevators); got != 2 {
				t.Errorf("len(Elevators) = %d want 2", got)
			}
			wantRange := FloorRangeSnapshot{Min: 1, Max: 10}
			for _, e := range out.Elevators {
				if diff := cmp.Diff(wantRange, e.FloorRange); diff != "" {
					t.Errorf("FloorRange mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestGetVisibleElevators(t *testing.T) {
	cases := []struct {
		name      string
		floor     int
		wantErr   bool
		wantFloor int
		wantCount int
	}{
		{name: "interior viewer floor", floor: 5, wantFloor: 5, wantCount: 2},
		{name: "out of range", floor: 99, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupDefault(t)
			uc := NewGetVisibleElevators(f.repo, f.locker)
			out, err := uc.Execute(f.ctx, GetVisibleElevatorsInput{Floor: c.floor})
			if c.wantErr {
				if err == nil {
					t.Errorf("expected error, got out=%+v", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			type ck struct{ Floor, Count int }
			want := ck{Floor: c.wantFloor, Count: c.wantCount}
			got := ck{Floor: out.Floor, Count: len(out.Elevators)}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResetSimulation(t *testing.T) {
	cases := []struct {
		name             string
		input            ResetSimulationInput
		wantFloorRange   FloorRangeSnapshot
		wantElevatorIDs  []string
		wantInitialFloor map[string]int
	}{
		{
			name:           "default range and elevators",
			input:          ResetSimulationInput{},
			wantFloorRange: DefaultFloorRange,
			// 2 号機いることだけ確認（IDs は default に依存しないよう個数で）
		},
		{
			name: "custom range and single elevator",
			input: ResetSimulationInput{
				FloorRange: &FloorRangeSnapshot{Min: -2, Max: 20},
				Elevators:  []ElevatorInit{{ID: "ev-x", InitialFloor: -1}},
			},
			wantFloorRange:   FloorRangeSnapshot{Min: -2, Max: 20},
			wantElevatorIDs:  []string{"ev-x"},
			wantInitialFloor: map[string]int{"ev-x": -1},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			repo := memory.NewElevatorBankRepository()
			simClk := memory.NewSimulationClock()
			locker := &fakeLocker{}
			uc := NewResetSimulation(repo, simClk, locker, DefaultFloorRange, DefaultElevators)

			out, err := uc.Execute(ctx, c.input)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if diff := cmp.Diff(c.wantFloorRange, out.FloorRange); diff != "" {
				t.Errorf("FloorRange mismatch (-want +got):\n%s", diff)
			}
			bank, _ := repo.Find(ctx)
			for id, wantFloor := range c.wantInitialFloor {
				ev, ok := bank.Elevator(elevator.ElevatorID(id))
				if !ok {
					t.Fatalf("%s not registered", id)
				}
				if got := ev.CurrentFloor().Value(); got != wantFloor {
					t.Errorf("%s currentFloor = %d, want %d", id, got, wantFloor)
				}
			}
			if c.wantElevatorIDs == nil {
				// default は 2 号機のはず
				if got := len(out.Elevators); got != 2 {
					t.Errorf("len(Elevators) = %d want 2", got)
				}
			}
		})
	}
}

func TestPatchElevator(t *testing.T) {
	cases := []struct {
		name        string
		buildInput  func() PatchElevatorInput
		wantErr     bool
		wantOpState string
	}{
		{
			name: "stop",
			buildInput: func() PatchElevatorInput {
				v := "stopped"
				return PatchElevatorInput{ElevatorID: "ev-1", OperationState: &v}
			},
			wantOpState: "stopped",
		},
		{
			name: "resume",
			buildInput: func() PatchElevatorInput {
				v := "running"
				return PatchElevatorInput{ElevatorID: "ev-1", OperationState: &v}
			},
			wantOpState: "running",
		},
		{
			name:       "empty patch rejected",
			buildInput: func() PatchElevatorInput { return PatchElevatorInput{ElevatorID: "ev-1"} },
			wantErr:    true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupDefault(t)
			uc := NewPatchElevator(f.repo, f.locker)
			out, err := uc.Execute(f.ctx, c.buildInput())
			if c.wantErr {
				if err == nil {
					t.Errorf("expected error, got out=%+v", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if diff := cmp.Diff(c.wantOpState, out.OperationState); diff != "" {
				t.Errorf("operationState mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCancelHallCall(t *testing.T) {
	cases := []struct {
		name       string
		seedCall   bool
		callID     string // 空なら seed した call の id を使う
		wantErr    bool
		wantStatus string
	}{
		{name: "registered call gets canceled", seedCall: true, wantStatus: "canceled"},
		{name: "missing id returns error", callID: "nope", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupDefault(t)
			pressUC := NewPressHallButton(f.repo, f.locker, f.clock, f.idGen)
			cancelUC := NewCancelHallCall(f.repo, f.locker)

			callID := c.callID
			if c.seedCall {
				out, err := pressUC.Execute(f.ctx, PressHallButtonInput{Floor: 5, Direction: "up"})
				if err != nil {
					t.Fatalf("PressHallButton: %v", err)
				}
				callID = out.Call.ID
			}
			err := cancelUC.Execute(f.ctx, CancelHallCallInput{CallID: callID})
			if c.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("CancelHallCall: %v", err)
			}
			bank, _ := f.repo.Find(f.ctx)
			for _, h := range bank.HallCalls() {
				if string(h.ID()) != callID {
					continue
				}
				if diff := cmp.Diff(c.wantStatus, string(h.Status())); diff != "" {
					t.Errorf("status mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestResetSimulation_RewindsTickCounter(t *testing.T) {
	f := setupDefault(t)
	advance := NewAdvanceTick(f.repo, f.simClk, f.locker, f.clock)
	if _, err := advance.Execute(f.ctx); err != nil {
		t.Fatalf("advance: %v", err)
	}
	reset := NewResetSimulation(f.repo, f.simClk, f.locker, DefaultFloorRange, DefaultElevators)
	if _, err := reset.Execute(f.ctx, ResetSimulationInput{}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	tick, _ := f.simClk.Tick(f.ctx)
	if diff := cmp.Diff(0, tick); diff != "" {
		t.Errorf("tick after reset mismatch (-want +got):\n%s", diff)
	}
}
