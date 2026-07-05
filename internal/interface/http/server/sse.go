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
	// subscribe → 初期状態取得の順にすることで、取得と購読開始の隙間の tick を
	// 取りこぼさない（先に届いた分はチャネルにバッファされる）。
	id, ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(id)

	// SSE ヘッダを書く前に初期状態を確定させる。ヘッダ送信後に失敗すると
	// 200 の空ストリームになり、EventSource が無言で再接続し続けるため。
	initial, err := h.initialPayload(r.Context())
	if err != nil {
		http.Error(w, "initial state unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	_, _ = fmt.Fprintf(w, "event: tick\ndata: %s\n\n", initial)
	flusher.Flush()

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

func (h *SSEHandler) initialPayload(ctx context.Context) ([]byte, error) {
	out, err := h.getState.Execute(ctx)
	if err != nil {
		return nil, err
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
	return json.Marshal(resp)
}
