package hydra

import (
	"sync"
	"time"
)

const (
	DRIVING_NONE         = 0
	DRIVING_OPEN_CTL     = 1
	DRIVING_OPEN_TO_END  = 2
	DRIVING_CLOSE_CTL    = 3
	DRIVING_CLOSE_TO_END = 4
)

type Sim struct {
	pos      int
	simError string

	drivingMode    int
	lastDrivingCmd time.Time

	lock sync.Mutex
}

const FULLY_OPEN = 100

func (s *Sim) Start() error {
	go s.drivingRoutine()
	return nil
}

func (s *Sim) drivingRoutine() {
	for {
		time.Sleep(500 * time.Millisecond)
		s.doDriving()
	}
}

func (s *Sim) doDriving() {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.drivingMode == DRIVING_OPEN_CTL || s.drivingMode == DRIVING_CLOSE_CTL {
		if time.Since(s.lastDrivingCmd) > 1500*time.Millisecond {
			// no command -> idle
			s.drivingMode = DRIVING_NONE
			return
		}
	}

	switch s.drivingMode {
	case DRIVING_OPEN_CTL:
		fallthrough
	case DRIVING_OPEN_TO_END:
		if s.pos == FULLY_OPEN {
			s.drivingMode = DRIVING_NONE
			return
		}

		s.pos++

	case DRIVING_CLOSE_CTL:
		fallthrough
	case DRIVING_CLOSE_TO_END:
		if s.pos == 0 {
			s.drivingMode = DRIVING_NONE
			return
		}

		s.pos--
	}
}

func (s *Sim) Status() (HydraStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.makeStatus(), nil
}

func (s *Sim) Open(clientTime time.Time) (HydraStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.pos == FULLY_OPEN {
		// Already open
		return s.makeStatus(), nil
	}

	s.lastDrivingCmd = time.Now()
	s.drivingMode = DRIVING_OPEN_CTL

	return s.makeStatus(), nil
}

func (s *Sim) Close(clientTime time.Time) (HydraStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.pos == 0 {
		// Already closed
		return s.makeStatus(), nil
	}

	s.lastDrivingCmd = time.Now()
	s.drivingMode = DRIVING_CLOSE_CTL

	return s.makeStatus(), nil
}

func (s *Sim) OpenToEnd(clientTime time.Time) (HydraStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.pos == FULLY_OPEN {
		// Already open
		return s.makeStatus(), nil
	}

	s.lastDrivingCmd = time.Now()
	s.drivingMode = DRIVING_OPEN_TO_END

	return s.makeStatus(), nil
}

func (s *Sim) CloseToEnd(clientTime time.Time) (HydraStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.pos == 0 {
		// Already closed
		return s.makeStatus(), nil
	}

	s.lastDrivingCmd = time.Now()
	s.drivingMode = DRIVING_CLOSE_TO_END

	return s.makeStatus(), nil
}

func (s *Sim) Stop(clientTime time.Time) (HydraStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.lastDrivingCmd = time.Now()
	s.drivingMode = DRIVING_NONE

	return s.makeStatus(), nil
}

func (s *Sim) SimError(inErrorState bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if inErrorState {
		s.drivingMode = DRIVING_NONE
		s.simError = "Oh nein, ein Fehler!"
	} else {
		s.simError = ""
	}
}

func (s *Sim) makeStatus() HydraStatus {
	posStr := ""
	if s.pos == 0 {
		posStr = POSITION_CLOSED
	} else if s.pos == FULLY_OPEN {
		posStr = POSITION_OPEN
	} else {
		posStr = POSITION_INBETWEEN
	}

	statusStr := ""
	if s.simError == "" {
		if s.drivingMode == DRIVING_NONE {
			statusStr = STATUS_IDLE
		} else {
			statusStr = STATUS_DRIVING
		}
	} else {
		statusStr = STATUS_ERROR
	}

	return HydraStatus{
		Status:   statusStr,
		Position: posStr,
		Error:    s.simError,
	}
}
