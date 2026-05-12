package memory

import "context"

type SimulationClock struct {
	tick int
}

func NewSimulationClock() *SimulationClock { return &SimulationClock{} }

func (c *SimulationClock) Tick(_ context.Context) (int, error) { return c.tick, nil }
func (c *SimulationClock) Advance(_ context.Context) error     { c.tick++; return nil }
func (c *SimulationClock) Reset(_ context.Context) error       { c.tick = 0; return nil }
