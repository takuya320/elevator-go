package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"elevator-go/internal/interface/http/oapi"
	"elevator-go/internal/usecase"
)

// 接続直後に現状を 1 回送ってから tick イベントを流す。
// 初期状態がないと、最初の tick 到着まで（最大 interval ms）UI が空のままになる。
type SSEHandler struct {
	broadcaster *Broadcaster
	getState    *usecase.GetState
}

func NewSSEHandler(b *Broadcaster, getState *usecase.GetState) *SSEHandler {
	return &SSEHandler{broadcaster: b, getState: getState}
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	id, ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(id)

	if err := h.writeInitial(r.Context(), w, flusher); err != nil {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case payload, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: tick\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

func (h *SSEHandler) writeInitial(ctx context.Context, w http.ResponseWriter, flusher http.Flusher) error {
	out, err := h.getState.Execute(ctx)
	if err != nil {
		return err
	}
	resp := oapi.SimulationTickResponse{
		Tick:      out.Tick,
		Elevators: make([]oapi.Elevator, 0, len(out.Elevators)),
		// 接続直後の初期状態にイベント履歴は含めない（クライアント側でログを蓄積）。
		Events: []oapi.SimulationEvent{},
	}
	for _, e := range out.Elevators {
		resp.Elevators = append(resp.Elevators, elevatorToOAPI(e))
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "event: tick\ndata: %s\n\n", payload)
	flusher.Flush()
	return nil
}
