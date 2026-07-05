package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"elevator-go/internal/domain/elevator"
	"elevator-go/internal/infrastructure/persistence/memory"
	"elevator-go/internal/interface/http/oapi"
)

// ドメイン sentinel → HTTP の対応はここ一カ所に集約する。
// ハンドラは writeError を呼ぶだけ。
var errorMapping = []struct {
	is     error
	status int
	code   oapi.ErrorCode
}{
	{elevator.ErrInvalidFloor, http.StatusBadRequest, oapi.OUTOFRANGE},
	{elevator.ErrInvalidDestinationFloor, http.StatusBadRequest, oapi.OUTOFRANGE},
	{elevator.ErrInvalidHallCallDirection, http.StatusBadRequest, oapi.INVALIDDIRECTION},
	{elevator.ErrInvalidDirection, http.StatusBadRequest, oapi.INVALIDREQUEST},
	{elevator.ErrInvalidDoorState, http.StatusBadRequest, oapi.INVALIDREQUEST},
	{elevator.ErrInvalidOperationState, http.StatusBadRequest, oapi.INVALIDREQUEST},
	{elevator.ErrElevatorAlreadyExists, http.StatusBadRequest, oapi.INVALIDREQUEST},
	{elevator.ErrElevatorNotFound, http.StatusNotFound, oapi.ELEVATORNOTFOUND},
	{elevator.ErrHallCallNotFound, http.StatusNotFound, oapi.CALLNOTFOUND},
	{elevator.ErrElevatorNotRunning, http.StatusConflict, oapi.INVALIDSTATE},
	{elevator.ErrNoAvailableElevator, http.StatusConflict, oapi.INVALIDSTATE},
	{elevator.ErrInvalidElevatorID, http.StatusBadRequest, oapi.INVALIDREQUEST},
	{elevator.ErrInvalidBuildingSpec, http.StatusBadRequest, oapi.INVALIDREQUEST},
	{memory.ErrNotInitialized, http.StatusConflict, oapi.INVALIDSTATE},
}

// 未知エラーは 500 INTERNAL に潰しつつログには残す。
func writeError(w http.ResponseWriter, err error) {
	for _, m := range errorMapping {
		if errors.Is(err, m.is) {
			writeErrorJSON(w, m.status, m.code, err.Error())
			return
		}
	}
	slog.Error("unhandled error", "err", err)
	writeErrorJSON(w, http.StatusInternalServerError, oapi.INTERNAL, "internal error")
}

func writeBadRequest(w http.ResponseWriter, msg string) {
	writeErrorJSON(w, http.StatusBadRequest, oapi.INVALIDREQUEST, msg)
}

func writeErrorJSON(w http.ResponseWriter, status int, code oapi.ErrorCode, msg string) {
	body := oapi.ErrorResponse{}
	body.Error.Code = code
	body.Error.Message = msg
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode response", "err", err)
	}
}
