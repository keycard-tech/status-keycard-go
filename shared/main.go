package main

import "errors"

func main() {}

type api int

const (
	none api = iota
	flowAPI
	sessionAPI
)

func checkAPIMutualExclusion(requestedAPI api) error {
	switch requestedAPI {
	case flowAPI:
		if globalRPCServer != nil {
			return errors.New("not allowed to start flow API when session API is being used")
		}
	case sessionAPI:
		if globalFlow != nil {
			return errors.New("not allowed to start session API when flow API is being used")
		}
	default:
		panic("Unknown API")
	}

	return nil
}
