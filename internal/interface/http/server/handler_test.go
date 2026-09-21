package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"elevator-go/internal/infrastructure/persistence/memory"
	syncpkg "elevator-go/internal/infrastructure/sync"
	"elevator-go/internal/interface/http/oapi"
	"elevator-go/internal/interface/http/server"
	"elevator-go/internal/usecase"
)

type fakeClock struct{ t time.Time }

func (f fakeClock) Now() time.Time { return f.t }

type fakeID struct{ seq int }

func (f *fakeID) NewID() string {
	f.seq++
	return fmt.Sprintf("call-%d", f.seq)
}

// 実依存（in-memory repo / mutex locker / fake clock & id）で完全配線したルータを返す。
// usecase 層と同じ部品を使うため Handler が usecase の動作を正しく取りまとめているかを検証できる。
func newRouter(t *testing.T) http.Handler {
	t.Helper()
	repo := memory.NewElevatorBankRepository()
	simClk := memory.NewSimulationClock()
	locker := syncpkg.NewMutexLocker()
	clock := fakeClock{t: time.Unix(1700000000, 0).UTC()}
	idGen := &fakeID{}

	advance := usecase.NewAdvanceTick(repo, simClk, locker, clock)
	getState := usecase.NewGetState(repo, simClk, locker)

	deps := server.HandlerDeps{
		PressHallButton:     usecase.NewPressHallButton(repo, locker, clock, idGen),
		PressCarButton:      usecase.NewPressCarButton(repo, locker),
		AdvanceTick:         advance,
		GetVisibleElevators: usecase.NewGetVisibleElevators(repo, locker),
		ResetSimulation:     usecase.NewResetSimulation(repo, simClk, locker, usecase.DefaultFloorRange, usecase.DefaultElevators),
		PatchElevator:       usecase.NewPatchElevator(repo, locker),
		CancelHallCall:      usecase.NewCancelHallCall(repo, locker),
		OpenDoor:            usecase.NewOpenDoor(repo, locker),
		CloseDoor:           usecase.NewCloseDoor(repo, locker),
		ListElevators:       usecase.NewListElevators(repo, locker),
		GetElevator:         usecase.NewGetElevator(repo, locker),
		AddElevator:         usecase.NewAddElevator(repo, locker),
		ListHallCalls:       usecase.NewListHallCalls(repo, locker),
		ListFloorHallCalls:  usecase.NewListFloorHallCalls(repo, locker),
	}

	if _, err := deps.ResetSimulation.Execute(context.Background(), usecase.ResetSimulationInput{}); err != nil {
		t.Fatalf("seed reset: %v", err)
	}

	bc := server.NewBroadcaster()
	sse := server.NewSSEHandler(bc, getState)

	router, err := server.NewRouter(server.NewHandler(deps), sse, nil)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return router
}

// Issue a request through the router; body is JSON-marshaled if non-nil.
func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var br io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		br = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, br)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return out
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	body := decode[oapi.ErrorResponse](t, rec)
	return string(body.Error.Code)
}

// --- happy path tests ---------------------------------------------------

func TestHandler_GetVisibleElevators(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodGet, "/floors/5/elevators", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := decode[oapi.FloorElevatorsResponse](t, rec)
	type ck struct {
		Floor int
		Count int
	}
	want := ck{Floor: 5, Count: 2}
	got := ck{Floor: body.Floor, Count: len(body.Elevators)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("body mismatch (-want +got):\n%s", diff)
	}
}

func TestHandler_PressHallButton_NewCall_Returns201Assigned(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := decode[oapi.HallCall](t, rec)
	type ck struct {
		Status      string
		HasAssigned bool
	}
	want := ck{Status: "assigned", HasAssigned: true}
	got := ck{Status: string(body.Status), HasAssigned: body.AssignedElevatorId != nil}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("hall call mismatch (-want +got):\n%s", diff)
	}
}

func TestHandler_PressHallButton_Idempotent_Returns200(t *testing.T) {
	h := newRouter(t)
	first := decode[oapi.HallCall](t, do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"}))
	rec := do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d want 200", rec.Code)
	}
	second := decode[oapi.HallCall](t, rec)
	if diff := cmp.Diff(first.Id, second.Id); diff != "" {
		t.Errorf("idempotent should return same id (-first +second):\n%s", diff)
	}
}

func TestHandler_PressCarButton_Returns201(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodPost, "/elevators/ev-1/car-calls", map[string]int{"destinationFloor": 7})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_AdvanceTick_IncrementsAndIncludesEvents(t *testing.T) {
	h := newRouter(t)
	// hall-call を発行して mutation event を溜めておく
	do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"})
	rec := do(t, h, http.MethodPost, "/simulation/tick", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := decode[oapi.SimulationTickResponse](t, rec)
	hasRequested := false
	for _, e := range body.Events {
		if string(e.Type) == "hall_call.requested" {
			hasRequested = true
		}
	}
	type ck struct {
		Tick         int
		HasRequested bool
		HasEvents    bool
	}
	want := ck{Tick: 1, HasRequested: true, HasEvents: true}
	got := ck{Tick: body.Tick, HasRequested: hasRequested, HasEvents: len(body.Events) > 0}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("tick body mismatch (-want +got):\n%s\nevents=%+v", diff, body.Events)
	}
}

func TestHandler_FullScenario_HallCallToServed(t *testing.T) {
	h := newRouter(t)
	// 5F up → ev-1 が 1F から assigned
	do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"})
	for range 4 {
		do(t, h, http.MethodPost, "/simulation/tick", nil)
	}
	rec := do(t, h, http.MethodGet, "/floors/5/elevators", nil)
	body := decode[oapi.FloorElevatorsResponse](t, rec)
	var ev1 oapi.VisibleElevator
	found := false
	for _, e := range body.Elevators {
		if e.Id == "ev-1" {
			ev1, found = e, true
			break
		}
	}
	if !found {
		t.Fatal("ev-1 not found")
	}
	type ck struct {
		Floor  int
		Status string
	}
	want := ck{Floor: 5, Status: "arrived"}
	got := ck{Floor: ev1.CurrentFloor, Status: string(ev1.VisibleStatus)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ev-1 visible state mismatch (-want +got):\n%s", diff)
	}
}

// --- admin endpoints ----------------------------------------------------

func TestHandler_StopThenResume(t *testing.T) {
	h := newRouter(t)
	cases := []struct {
		name string
		path string
		want string
	}{
		{"stop", "/elevators/ev-1/stop", "stopped"},
		{"resume", "/elevators/ev-1/resume", "running"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := decode[oapi.Elevator](t, do(t, h, http.MethodPost, c.path, nil))
			if diff := cmp.Diff(c.want, string(body.OperationState)); diff != "" {
				t.Errorf("operationState mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandler_PatchElevator_OperationState(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodPatch, "/elevators/ev-1", map[string]string{"operationState": "maintenance"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := decode[oapi.Elevator](t, rec)
	if diff := cmp.Diff("maintenance", string(body.OperationState)); diff != "" {
		t.Errorf("operationState mismatch (-want +got):\n%s", diff)
	}
}

func TestHandler_DoorOpenAndClose(t *testing.T) {
	h := newRouter(t)
	cases := []struct {
		name string
		path string
		want struct {
			DoorState string
			HoldOpen  bool
		}
	}{
		{
			name: "open",
			path: "/elevators/ev-1/doors/open",
			want: struct {
				DoorState string
				HoldOpen  bool
			}{DoorState: "open", HoldOpen: true},
		},
		{
			name: "close",
			path: "/elevators/ev-1/doors/close",
			want: struct {
				DoorState string
				HoldOpen  bool
			}{DoorState: "closed", HoldOpen: false},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := decode[oapi.Elevator](t, do(t, h, http.MethodPost, c.path, nil))
			got := struct {
				DoorState string
				HoldOpen  bool
			}{DoorState: string(body.DoorState), HoldOpen: body.DoorHoldOpen}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("door state mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandler_CancelHallCall(t *testing.T) {
	h := newRouter(t)
	hc := decode[oapi.HallCall](t, do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"}))
	cases := []struct {
		name string
		path string
		want int
	}{
		{"first delete returns 204", "/hall-calls/" + hc.Id, http.StatusNoContent},
		{"missing id returns 404", "/hall-calls/no-such-id", http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, h, http.MethodDelete, c.path, nil)
			if diff := cmp.Diff(c.want, rec.Code); diff != "" {
				t.Errorf("status mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandler_Reset_RestoresDefaults(t *testing.T) {
	h := newRouter(t)
	// 状態を変更しておく
	do(t, h, http.MethodPost, "/elevators/ev-1/car-calls", map[string]int{"destinationFloor": 7})
	for range 3 {
		do(t, h, http.MethodPost, "/simulation/tick", nil)
	}
	rec := do(t, h, http.MethodPost, "/simulation/reset", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d", rec.Code)
	}
	body := decode[oapi.SimulationResetResponse](t, rec)
	if diff := cmp.Diff("reset", string(body.Status)); diff != "" {
		t.Errorf("status mismatch (-want +got):\n%s", diff)
	}
	tick := decode[oapi.SimulationTickResponse](t, do(t, h, http.MethodPost, "/simulation/tick", nil))
	if diff := cmp.Diff(1, tick.Tick); diff != "" {
		t.Errorf("tick after reset+1 mismatch (-want +got):\n%s", diff)
	}
}

// --- error mapping ------------------------------------------------------

func TestHandler_ErrorMapping(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
		wantCode   string
	}{
		{
			name:   "top floor up → INVALID_DIRECTION",
			method: http.MethodPost, path: "/floors/10/hall-calls",
			body:       map[string]string{"direction": "up"},
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_DIRECTION",
		},
		{
			name:   "out-of-range floor → OUT_OF_RANGE",
			method: http.MethodPost, path: "/floors/99/hall-calls",
			body:       map[string]string{"direction": "up"},
			wantStatus: http.StatusBadRequest, wantCode: "OUT_OF_RANGE",
		},
		{
			name:   "car-call out of range → OUT_OF_RANGE",
			method: http.MethodPost, path: "/elevators/ev-1/car-calls",
			body:       map[string]int{"destinationFloor": 99},
			wantStatus: http.StatusBadRequest, wantCode: "OUT_OF_RANGE",
		},
		{
			name:   "unknown elevator → ELEVATOR_NOT_FOUND",
			method: http.MethodPost, path: "/elevators/nope/car-calls",
			body:       map[string]int{"destinationFloor": 5},
			wantStatus: http.StatusNotFound, wantCode: "ELEVATOR_NOT_FOUND",
		},
		{
			name:   "missing hall call → CALL_NOT_FOUND",
			method: http.MethodDelete, path: "/hall-calls/missing",
			body:       nil,
			wantStatus: http.StatusNotFound, wantCode: "CALL_NOT_FOUND",
		},
		{
			// json.Marshal が "not-json-but-string" として送出 → 受け側は struct への decode に失敗 → 400 INVALID_REQUEST。
			name:   "non-object JSON body → INVALID_REQUEST",
			method: http.MethodPost, path: "/floors/5/hall-calls",
			body:       "not-json-but-string",
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
		},
		{
			// opening / closing は API enum 予約のみ。PATCH では受け付けない。
			name:   "patch reserved doorState → INVALID_REQUEST",
			method: http.MethodPatch, path: "/elevators/ev-1",
			body:       map[string]string{"doorState": "opening"},
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
		},
		{
			name:   "patch unknown operationState → INVALID_REQUEST",
			method: http.MethodPatch, path: "/elevators/ev-1",
			body:       map[string]string{"operationState": "banana"},
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
		},
		{
			name:   "patch unknown direction → INVALID_REQUEST",
			method: http.MethodPatch, path: "/elevators/ev-1",
			body:       map[string]string{"direction": "sideways"},
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
		},
		{
			name:   "reset duplicate elevator id → INVALID_REQUEST",
			method: http.MethodPost, path: "/simulation/reset",
			body: map[string]any{"elevators": []map[string]any{
				{"id": "ev-1", "initialFloor": 1},
				{"id": "ev-1", "initialFloor": 2},
			}},
			wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST",
		},
	}
	type ck struct {
		Status int
		Code   string
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newRouter(t)
			rec := do(t, h, c.method, c.path, c.body)
			want := ck{Status: c.wantStatus, Code: c.wantCode}
			got := ck{Status: rec.Code, Code: errorCode(t, rec)}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("error mapping mismatch (-want +got):\n%s\nbody=%s", diff, rec.Body.String())
			}
		})
	}
}

func TestHandler_BadJSON_400_INVALID_REQUEST(t *testing.T) {
	h := newRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/floors/5/hall-calls", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	type ck struct {
		Status int
		Code   string
	}
	want := ck{Status: http.StatusBadRequest, Code: "INVALID_REQUEST"}
	got := ck{Status: rec.Code, Code: errorCode(t, rec)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("error mapping mismatch (-want +got):\n%s", diff)
	}
}

// --- 静的・spec ---------------------------------------------------------

func TestHandler_OpenAPISpec(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodGet, "/openapi.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Content-Type = %s", rec.Header().Get("Content-Type"))
	}
}

func TestHandler_AdvanceTick_ReassignsCallWhenAssigneeStops(t *testing.T) {
	h := newRouter(t)
	created := decode[oapi.HallCall](t, do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"}))
	if created.AssignedElevatorId == nil || *created.AssignedElevatorId != "ev-1" {
		t.Fatalf("initial assignee = %v want ev-1", created.AssignedElevatorId)
	}
	if rec := do(t, h, http.MethodPost, "/elevators/ev-1/stop", nil); rec.Code != http.StatusOK {
		t.Fatalf("stop status = %d", rec.Code)
	}

	body := decode[oapi.SimulationTickResponse](t, do(t, h, http.MethodPost, "/simulation/tick", nil))
	if got := len(body.HallCalls); got != 1 {
		t.Fatalf("len(HallCalls) = %d want 1", got)
	}
	type ck struct {
		Status   string
		Assignee string
		HasEvent bool
	}
	got := ck{Status: string(body.HallCalls[0].Status)}
	if a := body.HallCalls[0].AssignedElevatorId; a != nil {
		got.Assignee = *a
	}
	for _, e := range body.Events {
		if string(e.Type) == "hall_call.reassigned" {
			got.HasEvent = true
		}
	}
	want := ck{Status: "assigned", Assignee: "ev-2", HasEvent: true}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("reassignment mismatch (-want +got):\n%s\nevents=%+v", diff, body.Events)
	}
}

func TestHandler_AdvanceTick_KeepsHallCallVisibleWhenAllStopped(t *testing.T) {
	h := newRouter(t)
	do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"})
	for _, id := range []string{"ev-1", "ev-2"} {
		if rec := do(t, h, http.MethodPost, "/elevators/"+id+"/stop", nil); rec.Code != http.StatusOK {
			t.Fatalf("stop %s status = %d", id, rec.Code)
		}
	}

	body := decode[oapi.SimulationTickResponse](t, do(t, h, http.MethodPost, "/simulation/tick", nil))
	if got := len(body.HallCalls); got != 1 {
		t.Fatalf("len(HallCalls) = %d want 1", got)
	}
	if got, want := string(body.HallCalls[0].Status), "waiting"; got != want {
		t.Errorf("status = %s want %s", got, want)
	}
	if a := body.HallCalls[0].AssignedElevatorId; a != nil {
		t.Errorf("assignedElevatorId = %s want null", *a)
	}
}

func TestHandler_ListElevators(t *testing.T) {
	h := newRouter(t)
	body := decode[oapi.ElevatorsResponse](t, do(t, h, http.MethodGet, "/elevators", nil))
	ids := make([]string, 0, len(body.Elevators))
	for _, e := range body.Elevators {
		ids = append(ids, e.Id)
	}
	if diff := cmp.Diff([]string{"ev-1", "ev-2"}, ids); diff != "" {
		t.Errorf("ids mismatch (-want +got):\n%s", diff)
	}
}

func TestHandler_GetElevator(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodGet, "/elevators/ev-2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := decode[oapi.Elevator](t, rec)
	type ck struct {
		ID    string
		Floor int
	}
	if diff := cmp.Diff(ck{"ev-2", 10}, ck{body.Id, body.CurrentFloor}); diff != "" {
		t.Errorf("elevator mismatch (-want +got):\n%s", diff)
	}

	miss := do(t, h, http.MethodGet, "/elevators/ev-404", nil)
	if miss.Code != http.StatusNotFound {
		t.Errorf("unknown id status = %d want 404", miss.Code)
	}
	if got := errorCode(t, miss); got != "ELEVATOR_NOT_FOUND" {
		t.Errorf("code = %s want ELEVATOR_NOT_FOUND", got)
	}
}

func TestHandler_CreateElevator(t *testing.T) {
	h := newRouter(t)
	rec := do(t, h, http.MethodPost, "/elevators", map[string]any{"id": "ev-3", "initialFloor": 4})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	created := decode[oapi.Elevator](t, rec)
	type ck struct {
		ID        string
		Floor     int
		HomeFloor int
	}
	// homeFloor は initialFloor を引き継ぐ（NewElevator の既定）。
	if diff := cmp.Diff(ck{"ev-3", 4, 4}, ck{created.Id, created.CurrentFloor, created.HomeFloor}); diff != "" {
		t.Errorf("created mismatch (-want +got):\n%s", diff)
	}

	// 追加した号機は以降の配車対象になる。
	listed := decode[oapi.ElevatorsResponse](t, do(t, h, http.MethodGet, "/elevators", nil))
	if got := len(listed.Elevators); got != 3 {
		t.Errorf("len(elevators) = %d want 3", got)
	}

	cases := []struct {
		name     string
		body     map[string]any
		wantCode int
	}{
		{"duplicate id", map[string]any{"id": "ev-3", "initialFloor": 1}, http.StatusBadRequest},
		{"empty id", map[string]any{"id": "", "initialFloor": 1}, http.StatusBadRequest},
		{"floor out of range", map[string]any{"id": "ev-9", "initialFloor": 99}, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/elevators", c.body)
			if rec.Code != c.wantCode {
				t.Errorf("status = %d want %d body=%s", rec.Code, c.wantCode, rec.Body.String())
			}
		})
	}
}

func TestHandler_ListHallCalls(t *testing.T) {
	h := newRouter(t)
	do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"})
	do(t, h, http.MethodPost, "/floors/8/hall-calls", map[string]string{"direction": "down"})
	// 8F の呼びをキャンセルして status 違いを作る。
	listed := decode[oapi.HallCallsResponse](t, do(t, h, http.MethodGet, "/hall-calls", nil))
	var canceledID string
	for _, c := range listed.HallCalls {
		if c.Floor == 8 {
			canceledID = c.Id
		}
	}
	if canceledID == "" {
		t.Fatalf("8F の呼びが見つからない: %+v", listed.HallCalls)
	}
	do(t, h, http.MethodDelete, "/hall-calls/"+canceledID, nil)

	cases := []struct {
		name      string
		query     string
		wantCount int
	}{
		{"no filter returns canceled too", "", 2},
		{"status filter", "?status=assigned", 1},
		{"multiple statuses", "?status=assigned,canceled", 2},
		{"floor filter", "?floor=8", 1},
		{"floor with no calls", "?floor=2", 0},
		{"status and floor", "?status=assigned&floor=8", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := decode[oapi.HallCallsResponse](t, do(t, h, http.MethodGet, "/hall-calls"+c.query, nil))
			if got := len(body.HallCalls); got != c.wantCount {
				t.Errorf("len = %d want %d (%+v)", got, c.wantCount, body.HallCalls)
			}
		})
	}

	bad := do(t, h, http.MethodGet, "/hall-calls?status=bogus", nil)
	if bad.Code != http.StatusBadRequest {
		t.Errorf("unknown status = %d want 400", bad.Code)
	}
	if got := errorCode(t, bad); got != "INVALID_REQUEST" {
		t.Errorf("code = %s want INVALID_REQUEST", got)
	}
}

func TestHandler_ListFloorHallCalls(t *testing.T) {
	h := newRouter(t)
	do(t, h, http.MethodPost, "/floors/5/hall-calls", map[string]string{"direction": "up"})

	body := decode[oapi.FloorHallCallsResponse](t, do(t, h, http.MethodGet, "/floors/5/hall-calls", nil))
	if diff := cmp.Diff(5, body.Floor); diff != "" {
		t.Errorf("floor mismatch (-want +got):\n%s", diff)
	}
	if got := len(body.Calls); got != 1 {
		t.Fatalf("len(calls) = %d want 1", got)
	}

	empty := decode[oapi.FloorHallCallsResponse](t, do(t, h, http.MethodGet, "/floors/6/hall-calls", nil))
	if got := len(empty.Calls); got != 0 {
		t.Errorf("len(calls) = %d want 0", got)
	}

	// 階そのものがリソースなので範囲外は空ではなく 400。
	oor := do(t, h, http.MethodGet, "/floors/99/hall-calls", nil)
	if oor.Code != http.StatusBadRequest {
		t.Errorf("out of range status = %d want 400", oor.Code)
	}
	if got := errorCode(t, oor); got != "OUT_OF_RANGE" {
		t.Errorf("code = %s want OUT_OF_RANGE", got)
	}
}
