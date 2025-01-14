package internal

import "github.com/ebfe/scard"

type ReadersStates []scard.ReaderState

func (rs ReadersStates) Contains(reader string) bool {
	for _, state := range rs {
		if state.Reader == reader {
			return true
		}
	}
	return false
}

func (rs ReadersStates) Update() {
	for i := range rs {
		rs[i].CurrentState = rs[i].EventState
	}
}

func (rs ReadersStates) ReaderWithCardIndex() (int, bool) {
	for i := range rs {
		if rs[i].EventState&scard.StatePresent == 0 {
			continue
		}

		// NOTE: For now we only support one card at a time
		return i, true
	}

	return -1, false
}

func (rs *ReadersStates) Append(reader scard.ReaderState) {
	*rs = append(*rs, reader)
}

func (rs ReadersStates) ReaderHasCard(reader string) bool {
	for _, state := range rs {
		if state.Reader == reader && state.EventState&scard.StatePresent != 0 {
			return true
		}
	}
	return false
}
