//go:build windows

package notifyaudio

import (
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmmDLL      = windows.NewLazySystemDLL("winmm.dll")
	mciSendString = winmmDLL.NewProc("mciSendStringW")
)

func playMIDIFile(path string) error {
	alias := fmt.Sprintf("phgo_midi_%d", time.Now().UnixNano())
	escapedPath := strings.ReplaceAll(path, `"`, `""`)
	if err := mciCommand(fmt.Sprintf(`open "%s" type sequencer alias %s`, escapedPath, alias)); err != nil {
		return err
	}
	defer func() {
		_ = mciCommand("close " + alias)
	}()
	return mciCommand("play " + alias + " from 0 wait")
}

func mciCommand(command string) error {
	ptr, err := windows.UTF16PtrFromString(command)
	if err != nil {
		return err
	}
	ret, _, callErr := mciSendString.Call(uintptr(unsafe.Pointer(ptr)), 0, 0, 0)
	if ret != 0 {
		if callErr != windows.ERROR_SUCCESS && callErr != nil {
			return callErr
		}
		return fmt.Errorf("mci command failed: %s", command)
	}
	return nil
}
