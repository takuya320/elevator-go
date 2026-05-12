package elevator

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewElevatorID(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{name: "rejects empty", in: "", wantErr: ErrInvalidElevatorID},
		{name: "accepts non-empty", in: "ev-1", want: "ev-1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, err := NewElevatorID(c.in)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(c.want, id.String()); diff != "" {
				t.Errorf("id mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewHallCallID(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "rejects empty", in: "", wantErr: true},
		{name: "accepts non-empty", in: "call-1", wantErr: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewHallCallID(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}
