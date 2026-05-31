//go:build windows

package notifyaudio

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmmDLL      = windows.NewLazySystemDLL("winmm.dll")
	mciSendString = winmmDLL.NewProc("mciSendStringW")
	playMu        sync.Mutex
	currentAlias  string
)

func playAudioFile(path string) error {
	playMu.Lock()
	defer playMu.Unlock()

	if currentAlias != "" {
		_ = mciCommand("stop " + currentAlias)
		_ = mciCommand("close " + currentAlias)
		currentAlias = ""
	}

	alias := fmt.Sprintf("phgo_ready_%d", time.Now().UnixNano())
	escapedPath := strings.ReplaceAll(path, `"`, `""`)
	if err := mciCommand(fmt.Sprintf(`open "%s" type mpegvideo alias %s`, escapedPath, alias)); err != nil {
		return err
	}
	if err := mciCommand("play " + alias + " from 0"); err != nil {
		_ = mciCommand("close " + alias)
		return err
	}
	currentAlias = alias
	return nil
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
