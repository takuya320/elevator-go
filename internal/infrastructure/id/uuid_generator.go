package id

import "github.com/google/uuid"

type UUID struct{}

func (UUID) NewID() string { return uuid.NewString() }
