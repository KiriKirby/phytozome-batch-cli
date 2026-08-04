//go:build windows

package notifyaudio

import (
	"fmt"
	"math"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmmDLL     = windows.NewLazySystemDLL("winmm.dll")
	midiOutOpen  = winmmDLL.NewProc("midiOutOpen")
	midiOutClose = winmmDLL.NewProc("midiOutClose")
	midiOutReset = winmmDLL.NewProc("midiOutReset")
	midiOutShort = winmmDLL.NewProc("midiOutShortMsg")
)

func playMIDIBytes(data []byte) error {
	seq, err := parseSMF(data)
	if err != nil {
		return err
	}
	if len(seq.events) == 0 {
		return nil
	}
	handle, err := openMIDIOut()
	if err != nil {
		return err
	}
	defer func() {
		_ = resetMIDIOut(handle)
		_ = closeMIDIOut(handle)
	}()
	const defaultTempoMicrosPerQuarter = 500000
	tempo := defaultTempoMicrosPerQuarter
	var lastTick uint64
	for _, event := range seq.events {
		if event.tick > lastTick {
			time.Sleep(smfTickDuration(event.tick-lastTick, seq.division, tempo))
			lastTick = event.tick
		}
		if event.kind == smfEventTempo {
			eventTempo := event.tempoMicrosPerQuarter
			if eventTempo > 0 {
				tempo = eventTempo
			}
			continue
		}
		if err := sendMIDIShort(handle, event.status, event.data1, event.data2); err != nil {
			return err
		}
	}
	time.Sleep(80 * time.Millisecond)
	return nil
}

func openMIDIOut() (windows.Handle, error) {
	var handle windows.Handle
	const midiMapper = ^uint32(0)
	ret, _, callErr := midiOutOpen.Call(uintptr(unsafe.Pointer(&handle)), uintptr(midiMapper), 0, 0, 0)
	if ret == 0 {
		return handle, nil
	}
	ret, _, callErr = midiOutOpen.Call(uintptr(unsafe.Pointer(&handle)), 0, 0, 0, 0)
	if ret == 0 {
		return handle, nil
	}
	if callErr != windows.ERROR_SUCCESS && callErr != nil {
		return 0, callErr
	}
	return 0, fmt.Errorf("midiOutOpen failed with code %d", ret)
}

func closeMIDIOut(handle windows.Handle) error {
	ret, _, callErr := midiOutClose.Call(uintptr(handle))
	if ret == 0 {
		return nil
	}
	if callErr != windows.ERROR_SUCCESS && callErr != nil {
		return callErr
	}
	return fmt.Errorf("midiOutClose failed with code %d", ret)
}

func resetMIDIOut(handle windows.Handle) error {
	ret, _, callErr := midiOutReset.Call(uintptr(handle))
	if ret == 0 {
		return nil
	}
	if callErr != windows.ERROR_SUCCESS && callErr != nil {
		return callErr
	}
	return fmt.Errorf("midiOutReset failed with code %d", ret)
}

func sendMIDIShort(handle windows.Handle, status, data1, data2 byte) error {
	msg := uintptr(status) | uintptr(data1)<<8 | uintptr(data2)<<16
	ret, _, callErr := midiOutShort.Call(uintptr(handle), msg)
	if ret == 0 {
		return nil
	}
	if callErr != windows.ERROR_SUCCESS && callErr != nil {
		return callErr
	}
	return fmt.Errorf("midiOutShortMsg failed with code %d", ret)
}

func smfTickDuration(ticks uint64, division uint16, tempoMicrosPerQuarter int) time.Duration {
	if ticks == 0 {
		return 0
	}
	if division&0x8000 == 0 {
		if division == 0 || tempoMicrosPerQuarter <= 0 {
			return 0
		}
		seconds := float64(ticks) * float64(tempoMicrosPerQuarter) / float64(division) / 1_000_000
		return time.Duration(seconds * float64(time.Second))
	}
	fpsCode := int8(byte(division >> 8))
	fps := float64(-fpsCode)
	if fpsCode == -29 {
		fps = 29.97
	}
	ticksPerFrame := float64(division & 0xFF)
	if fps <= 0 || ticksPerFrame <= 0 || math.IsNaN(fps) {
		return 0
	}
	seconds := float64(ticks) / (fps * ticksPerFrame)
	return time.Duration(seconds * float64(time.Second))
}
