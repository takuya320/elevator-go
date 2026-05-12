package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"elevator-go/internal/interface/http/oapi"
	"elevator-go/internal/usecase"
)

type AutoTicker struct {
	interval    time.Duration
	advance     *usecase.AdvanceTick
	broadcaster *Broadcaster
}

func NewAutoTicker(interval time.Duration, advance *usecase.AdvanceTick, b *Broadcaster) *AutoTicker {
	return &AutoTicker{interval: interval, advance: advance, broadcaster: b}
}

// ctx が Done になるまで間隔ごとに tick を進めて全クライアントへ配信する。
func (t *AutoTicker) Run(ctx context.Context) {
	tk := time.NewTicker(t.interval)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			out, err := t.advance.Execute(ctx)
			if err != nil {
				slog.Error("auto tick failed", "err", err)
				continue
			}
			resp := oapi.SimulationTickResponse{
				Tick:      out.Tick,
				Elevators: make([]oapi.Elevator, 0, len(out.Elevators)),
				Events:    eventsToOAPI(out.Events),
			}
			for _, e := range out.Elevators {
				resp.Elevators = append(resp.Elevators, elevatorToOAPI(e))
			}
			payload, err := json.Marshal(resp)
			if err != nil {
				slog.Error("auto tick marshal failed", "err", err)
				continue
			}
			t.broadcaster.Publish(payload)
		}
	}
}
