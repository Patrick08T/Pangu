package agent

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

func ProcessIdentity(pid int) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	stamp, err := processStamp(handle)
	if err != nil {
		return "", err
	}
	var session uint32
	if err := windows.ProcessIdToSessionId(uint32(pid), &session); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", stamp, session), nil
}

func processStamp(handle windows.Handle) (string, error) {
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x%08x", created.HighDateTime, created.LowDateTime), nil
}

func ConfirmStopped(pid int, stamp, scope string) (bool, error) {
	if pid <= 0 || stamp == "" {
		return false, nil
	}
	created, session, valid := strings.Cut(stamp, "/")
	ownerSession, err := strconv.ParseUint(session, 10, 32)
	if !valid || err != nil {
		return false, nil
	}
	var currentSession uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &currentSession); err != nil {
		return false, err
	}
	if uint64(currentSession) != ownerSession {
		return false, nil
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err == nil {
		defer windows.CloseHandle(handle)
		current, err := processStamp(handle)
		if err != nil {
			return false, err
		}
		status, err := windows.WaitForSingleObject(handle, 0)
		if err != nil {
			return false, err
		}
		if current == created && status != windows.WAIT_OBJECT_0 {
			return false, nil
		}
	} else if !errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, err
	}
	if scope == "" {
		return true, nil
	}
	if _, err := uuid.Parse(scope); err != nil {
		return false, err
	}
	name, _ := windows.UTF16PtrFromString(`Local\Pangu-` + scope)
	open := windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenJobObjectW")
	if err := open.Find(); err != nil {
		return false, err
	}
	job, _, callErr := open.Call(0x000C, 0, uintptr(unsafe.Pointer(name)))
	if job == 0 {
		if errors.Is(callErr, windows.ERROR_FILE_NOT_FOUND) {
			return true, nil
		}
		return false, callErr
	}
	defer windows.CloseHandle(windows.Handle(job))
	err = stopCodexJob(windows.Handle(job))
	return err == nil, err
}
