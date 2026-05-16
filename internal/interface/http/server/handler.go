package server

import (
	"encoding/json"
	"net/http"

	"elevator-go/internal/interface/http/oapi"
	"elevator-go/internal/usecase"
)

type HandlerDeps struct {
	PressHallButton     *usecase.PressHallButton
	PressCarButton      *usecase.PressCarButton
	AdvanceTick         *usecase.AdvanceTick
	GetVisibleElevators *usecase.GetVisibleElevators
	ResetSimulation     *usecase.ResetSimulation
	PatchElevator       *usecase.PatchElevator
	CancelHallCall      *usecase.CancelHallCall
	OpenDoor            *usecase.OpenDoor
	CloseDoor           *usecase.CloseDoor
}

// 上書きしないメソッドは oapi.Unimplemented により 501 を返す。
type Handler struct {
	oapi.Unimplemented
	deps HandlerDeps
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{deps: deps}
}

func (h *Handler) GetVisibleElevators(w http.ResponseWriter, r *http.Request, floor oapi.FloorPath) {
	out, err := h.deps.GetVisibleElevators.Execute(r.Context(), usecase.GetVisibleElevatorsInput{Floor: floor})
	if err != nil {
		writeError(w, err)
		return
	}
	resp := oapi.FloorElevatorsResponse{
		Floor:     out.Floor,
		Elevators: make([]oapi.VisibleElevator, 0, len(out.Elevators)),
	}
	for _, v := range out.Elevators {
		resp.Elevators = append(resp.Elevators, visibleElevatorToOAPI(v))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) PressHallButton(w http.ResponseWriter, r *http.Request, floor oapi.FloorPath) {
	var body oapi.HallCallCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	out, err := h.deps.PressHallButton.Execute(r.Context(), usecase.PressHallButtonInput{
		Floor:     floor,
		Direction: string(body.Direction),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	// 既存の active call を返した場合は 200、新規作成なら 201。
	status := http.StatusCreated
	if !out.Created {
		status = http.StatusOK
	}
	writeJSON(w, status, hallCallToOAPI(out.Call))
}

func (h *Handler) PressCarButton(w http.ResponseWriter, r *http.Request, elevatorId oapi.ElevatorIdPath) {
	var body oapi.CarCallCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	if err := h.deps.PressCarButton.Execute(r.Context(), usecase.PressCarButtonInput{
		ElevatorID:       elevatorId,
		DestinationFloor: body.DestinationFloor,
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, oapi.CarCallResponse{
		ElevatorId:       elevatorId,
		DestinationFloor: body.DestinationFloor,
		Status:           oapi.Accepted,
	})
}

func (h *Handler) AdvanceTick(w http.ResponseWriter, r *http.Request) {
	out, err := h.deps.AdvanceTick.Execute(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	resp := oapi.SimulationTickResponse{
		Tick:      out.Tick,
		Elevators: make([]oapi.Elevator, 0, len(out.Elevators)),
		Events:    eventsToOAPI(out.Events),
	}
	for _, e := range out.Elevators {
		resp.Elevators = append(resp.Elevators, elevatorToOAPI(e))
	}
	writeJSON(w, http.StatusOK, resp)
}

// Stop / Resume は PatchElevator の薄いラッパー。
func (h *Handler) StopElevator(w http.ResponseWriter, r *http.Request, elevatorId oapi.ElevatorIdPath) {
	h.applyOperationState(w, r, elevatorId, "stopped")
}

func (h *Handler) ResumeElevator(w http.ResponseWriter, r *http.Request, elevatorId oapi.ElevatorIdPath) {
	h.applyOperationState(w, r, elevatorId, "running")
}

func (h *Handler) applyOperationState(w http.ResponseWriter, r *http.Request, elevatorId, state string) {
	out, err := h.deps.PatchElevator.Execute(r.Context(), usecase.PatchElevatorInput{
		ElevatorID:     elevatorId,
		OperationState: &state,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, elevatorToOAPI(*out))
}

func (h *Handler) PatchElevator(w http.ResponseWriter, r *http.Request, elevatorId oapi.ElevatorIdPath) {
	var body oapi.ElevatorPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	in := usecase.PatchElevatorInput{ElevatorID: elevatorId}
	in.CurrentFloor = body.CurrentFloor
	if body.Direction != nil {
		s := string(*body.Direction)
		in.Direction = &s
	}
	if body.DoorState != nil {
		s := string(*body.DoorState)
		in.DoorState = &s
	}
	if body.OperationState != nil {
		s := string(*body.OperationState)
		in.OperationState = &s
	}
	in.HomeFloor = body.HomeFloor
	in.AutoReturnEnabled = body.AutoReturnEnabled
	out, err := h.deps.PatchElevator.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, elevatorToOAPI(*out))
}

func (h *Handler) OpenDoor(w http.ResponseWriter, r *http.Request, elevatorId oapi.ElevatorIdPath) {
	out, err := h.deps.OpenDoor.Execute(r.Context(), elevatorId)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, elevatorToOAPI(*out))
}

func (h *Handler) CloseDoor(w http.ResponseWriter, r *http.Request, elevatorId oapi.ElevatorIdPath) {
	out, err := h.deps.CloseDoor.Execute(r.Context(), elevatorId)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, elevatorToOAPI(*out))
}

func (h *Handler) CancelHallCall(w http.ResponseWriter, r *http.Request, callId string) {
	if err := h.deps.CancelHallCall.Execute(r.Context(), usecase.CancelHallCallInput{CallID: callId}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ResetSimulation(w http.ResponseWriter, r *http.Request) {
	in := usecase.ResetSimulationInput{}
	if r.ContentLength != 0 {
		var body oapi.SimulationResetRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeBadRequest(w, "invalid JSON body")
			return
		}
		if body.FloorRange != nil {
			in.FloorRange = &usecase.FloorRangeSnapshot{
				Min: body.FloorRange.Min,
				Max: body.FloorRange.Max,
			}
		}
		if body.Elevators != nil {
			for _, e := range *body.Elevators {
				init := usecase.ElevatorInit{
					ID:           e.Id,
					InitialFloor: e.InitialFloor,
					HomeFloor:    e.HomeFloor,
				}
				if e.AutoReturnEnabled != nil {
					init.AutoReturnEnabled = *e.AutoReturnEnabled
				}
				in.Elevators = append(in.Elevators, init)
			}
		}
	}
	out, err := h.deps.ResetSimulation.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := oapi.SimulationResetResponse{
		Status:     oapi.Reset,
		FloorRange: floorRangeToOAPI(out.FloorRange),
		Elevators:  make([]oapi.ElevatorInit, 0, len(out.Elevators)),
	}
	for _, e := range out.Elevators {
		resp.Elevators = append(resp.Elevators, elevatorInitToOAPI(e))
	}
	writeJSON(w, http.StatusOK, resp)
}
