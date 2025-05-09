package hydra

import "time"

const STATUS_IDLE = "idle"
const STATUS_DRIVING = "driving"
const STATUS_ERROR = "error"

const POSITION_INBETWEEN = "inbetween"
const POSITION_CLOSED = "closed"
const POSITION_OPEN = "open"

type HydraStatus struct {
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
	Position string `json:"position,omitempty"`
}

type Inst interface {
	Status() (HydraStatus, error)
	Open(time time.Time) (HydraStatus, error)
	Close(time time.Time) (HydraStatus, error)
	OpenToEnd(time time.Time) (HydraStatus, error)
	CloseToEnd(time time.Time) (HydraStatus, error)
	Stop(time time.Time) (HydraStatus, error)

	SimError(inErrorState bool)
}
