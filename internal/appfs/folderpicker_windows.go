//go:build windows

package appfs

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	fileDialogOptionNoChangeDir     = 0x00000008
	fileDialogOptionPickFolders     = 0x00000020
	fileDialogOptionForceFileSystem = 0x00000040
	fileDialogOptionPathMustExist   = 0x00000800
	shellItemFileSystemPath         = 0x80058000
	hresultSFalse                   = 0x00000001
	hresultCancelled                = 0x800704C7
)

var (
	clsidFileOpenDialog = windows.GUID{Data1: 0xdc1c5a9c, Data2: 0xe88a, Data3: 0x4dde, Data4: [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
	iidIFileOpenDialog  = windows.GUID{Data1: 0xd57c7288, Data2: 0xd4ad, Data3: 0x4768, Data4: [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60}}
	iidIShellItem       = windows.GUID{Data1: 0x43826d1e, Data2: 0xe718, Data3: 0x42ee, Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}

	modOle32  = windows.NewLazySystemDLL("ole32.dll")
	modShell  = windows.NewLazySystemDLL("shell32.dll")
	modUser32 = windows.NewLazySystemDLL("user32.dll")

	procCoCreateInstance            = modOle32.NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = modShell.NewProc("SHCreateItemFromParsingName")
	procGetForegroundWindow         = modUser32.NewProc("GetForegroundWindow")
)

type iFileOpenDialog struct {
	vtbl *iFileOpenDialogVtbl
}

type iFileOpenDialogVtbl struct {
	queryInterface   uintptr
	addRef           uintptr
	release          uintptr
	show             uintptr
	setFileTypes     uintptr
	setFileTypeIndex uintptr
	getFileTypeIndex uintptr
	advise           uintptr
	unadvise         uintptr
	setOptions       uintptr
	getOptions       uintptr
	setDefaultFolder uintptr
	setFolder        uintptr
	getFolder        uintptr
	getCurrentSelect uintptr
	setFileName      uintptr
	getFileName      uintptr
	setTitle         uintptr
	setOkButtonLabel uintptr
	setFileNameLabel uintptr
	getResult        uintptr
	addPlace         uintptr
	setDefaultExt    uintptr
	close            uintptr
	setClientGuid    uintptr
	clearClientData  uintptr
	setFilter        uintptr
	getResults       uintptr
	getSelectedItems uintptr
}

type iShellItem struct {
	vtbl *iShellItemVtbl
}

type iShellItemVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
	bindToHandler  uintptr
	getParent      uintptr
	getDisplayName uintptr
	getAttributes  uintptr
	compare        uintptr
}

func selectFolderSystem(title string, defaultDir string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Select export folder"
	}
	selected, err := selectPathWindowsNative(title, defaultDir, true)
	if err == nil || errors.Is(err, ErrFolderSelectionCancelled) {
		return selected, err
	}
	return selectFolderWindowsPowerShell(title, defaultDir)
}

func selectFileSystem(title string, defaultDir string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Open file"
	}
	selected, err := selectPathWindowsNative(title, defaultDir, false)
	if err == nil || errors.Is(err, ErrFileSelectionCancelled) {
		return selected, err
	}
	return selectFileWindowsPowerShell(title, defaultDir)
}

func selectPathWindowsNative(title string, defaultDir string, pickFolder bool) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	coErr := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE)
	coInitialized := coErr == nil
	if coErr != nil {
		if errno, ok := coErr.(windows.Errno); ok && uintptr(errno) == hresultSFalse {
			coInitialized = true
		} else if errno, ok := coErr.(windows.Errno); !ok || uintptr(errno) != uintptr(windows.RPC_E_CHANGED_MODE) {
			return "", fmt.Errorf("initialize COM for Windows folder picker: %w", coErr)
		}
	}
	if coInitialized {
		defer windows.CoUninitialize()
	}

	var dialog *iFileOpenDialog
	if err := hresultError(procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		uintptr(windows.CLSCTX_INPROC_SERVER),
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)),
		uintptr(unsafe.Pointer(&dialog)),
	)); err != nil {
		return "", fmt.Errorf("create Windows folder picker: %w", err)
	}
	if dialog == nil {
		return "", fmt.Errorf("create Windows folder picker: empty dialog")
	}
	defer dialog.release()

	var options uint32
	if err := dialog.getOptions(&options); err != nil {
		return "", err
	}
	options |= fileDialogOptionForceFileSystem | fileDialogOptionPathMustExist | fileDialogOptionNoChangeDir
	if pickFolder {
		options |= fileDialogOptionPickFolders
	}
	if err := dialog.setOptions(options); err != nil {
		return "", err
	}
	if titlePtr, err := windows.UTF16PtrFromString(title); err == nil {
		if err := dialog.setTitle(titlePtr); err != nil {
			return "", err
		}
	}
	if folder, err := shellItemFromPath(defaultDir); err == nil && folder != nil {
		_ = dialog.setFolder(folder)
		folder.release()
	}

	if err := dialog.show(foregroundWindow()); err != nil {
		if isCancelledHResult(err) {
			if pickFolder {
				return "", ErrFolderSelectionCancelled
			}
			return "", ErrFileSelectionCancelled
		}
		return "", err
	}
	result, err := dialog.getResult()
	if err != nil {
		return "", err
	}
	defer result.release()
	path, err := result.fileSystemPath()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}

func selectFolderWindowsPowerShell(title string, defaultDir string) (string, error) {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		powershell, err = exec.LookPath("powershell")
		if err != nil {
			return "", err
		}
	}
	script := strings.Join([]string{
		`Add-Type -AssemblyName System.Windows.Forms`,
		`$dialog = New-Object System.Windows.Forms.FolderBrowserDialog`,
		`$dialog.Description = $args[0]`,
		`$dialog.SelectedPath = $args[1]`,
		`$dialog.ShowNewFolderButton = $true`,
		`$result = $dialog.ShowDialog()`,
		`if ($result -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($dialog.SelectedPath); exit 0 }`,
		`exit 2`,
	}, "; ")
	out, err := exec.Command(powershell, "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script, title, defaultDir).CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return "", ErrFolderSelectionCancelled
		}
		return "", fmt.Errorf("open Windows folder picker: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func selectFileWindowsPowerShell(title string, defaultDir string) (string, error) {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		powershell, err = exec.LookPath("powershell")
		if err != nil {
			return "", err
		}
	}
	script := strings.Join([]string{
		`Add-Type -AssemblyName System.Windows.Forms`,
		`$dialog = New-Object System.Windows.Forms.OpenFileDialog`,
		`$dialog.Title = $args[0]`,
		`$dialog.InitialDirectory = $args[1]`,
		`$dialog.CheckFileExists = $true`,
		`$dialog.CheckPathExists = $true`,
		`$dialog.Multiselect = $false`,
		`$result = $dialog.ShowDialog()`,
		`if ($result -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($dialog.FileName); exit 0 }`,
		`exit 2`,
	}, "; ")
	out, err := exec.Command(powershell, "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script, title, defaultDir).CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return "", ErrFileSelectionCancelled
		}
		return "", fmt.Errorf("open Windows file picker: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *iFileOpenDialog) release() {
	syscall.SyscallN(d.vtbl.release, uintptr(unsafe.Pointer(d)))
}

func (d *iFileOpenDialog) show(owner windows.Handle) error {
	return hresultError(syscall.SyscallN(d.vtbl.show, uintptr(unsafe.Pointer(d)), uintptr(owner)))
}

func (d *iFileOpenDialog) getOptions(options *uint32) error {
	return hresultError(syscall.SyscallN(d.vtbl.getOptions, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(options))))
}

func (d *iFileOpenDialog) setOptions(options uint32) error {
	return hresultError(syscall.SyscallN(d.vtbl.setOptions, uintptr(unsafe.Pointer(d)), uintptr(options)))
}

func (d *iFileOpenDialog) setTitle(title *uint16) error {
	return hresultError(syscall.SyscallN(d.vtbl.setTitle, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(title))))
}

func (d *iFileOpenDialog) setFolder(folder *iShellItem) error {
	return hresultError(syscall.SyscallN(d.vtbl.setFolder, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(folder))))
}

func (d *iFileOpenDialog) getResult() (*iShellItem, error) {
	var item *iShellItem
	if err := hresultError(syscall.SyscallN(d.vtbl.getResult, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&item)))); err != nil {
		return nil, err
	}
	return item, nil
}

func shellItemFromPath(path string) (*iShellItem, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var item *iShellItem
	if err := hresultError(procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&iidIShellItem)),
		uintptr(unsafe.Pointer(&item)),
	)); err != nil {
		return nil, err
	}
	return item, nil
}

func (i *iShellItem) release() {
	syscall.SyscallN(i.vtbl.release, uintptr(unsafe.Pointer(i)))
}

func (i *iShellItem) fileSystemPath() (string, error) {
	var raw *uint16
	if err := hresultError(syscall.SyscallN(i.vtbl.getDisplayName, uintptr(unsafe.Pointer(i)), shellItemFileSystemPath, uintptr(unsafe.Pointer(&raw)))); err != nil {
		return "", err
	}
	if raw == nil {
		return "", fmt.Errorf("Windows folder picker returned no path")
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(raw))
	return windows.UTF16PtrToString(raw), nil
}

func foregroundWindow() windows.Handle {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return windows.Handle(hwnd)
}

func hresultError(r1 uintptr, _ uintptr, _ error) error {
	if int32(r1) >= 0 {
		return nil
	}
	return windows.Errno(r1)
}

func isCancelledHResult(err error) bool {
	if err == nil {
		return false
	}
	if errno, ok := err.(windows.Errno); ok {
		return uintptr(errno) == hresultCancelled
	}
	return false
}
