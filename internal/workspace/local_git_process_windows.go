package workspace

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type localGitJob struct {
	handle windows.Handle
	once   sync.Once
	err    error
}

func newLocalGitJob() (*localGitJob, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	return &localGitJob{handle: handle}, nil
}

func startLocalGit(cmd *exec.Cmd) (func() error, error) {
	job, err := newLocalGitJob()
	if err != nil {
		return nil, err
	}
	return startLocalGitInJob(cmd, job)
}

func startLocalGitInJob(cmd *exec.Cmd, job *localGitJob) (func() error, error) {
	var err error
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED, HideWindow: true}
	cmd.Cancel = job.stop
	if err = cmd.Start(); err != nil {
		// No child started; closing the empty Job cannot imply a running orphan.
		return nil, errors.Join(err, windows.CloseHandle(job.handle))
	}
	if err = attachLocalGitProcess(cmd, job.handle); err != nil {
		killErr := cmd.Process.Kill()
		waitErr := cmd.Wait()
		reapErr := localGitReapError(cmd, errors.Join(killErr, waitErr))
		return nil, errors.Join(err, reapErr, job.stop())
	}
	return job.stop, nil
}

func attachLocalGitProcess(cmd *exec.Cmd, job windows.Handle) error {
	access := uint32(windows.PROCESS_SET_QUOTA | windows.PROCESS_TERMINATE | windows.PROCESS_SUSPEND_RESUME)
	process, err := windows.OpenProcess(access, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fmt.Errorf("open owned Git process: %w", err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		return fmt.Errorf("assign owned Git Job: %w", err)
	}
	resume := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")
	if err := resume.Find(); err != nil {
		return err
	}
	status, _, _ := resume.Call(uintptr(process))
	if status != 0 {
		return windows.NTStatus(status)
	}
	return nil
}

func (j *localGitJob) stop() error {
	j.once.Do(func() {
		j.err = windows.TerminateJobObject(j.handle, 1)
		if j.err == nil {
			j.err = waitLocalGitJob(j.handle)
		}
		j.err = localGitStopError(errors.Join(j.err, windows.CloseHandle(j.handle)))
	})
	return j.err
}

func waitLocalGitJob(handle windows.Handle) error {
	// JOBOBJECT_BASIC_ACCOUNTING_INFORMATION, fixed Windows ABI.
	var info struct {
		user, kernel, periodUser, periodKernel int64
		faults, total, active, terminated      uint32
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := windows.QueryInformationJobObject(handle, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
			return localGitStopError(err)
		}
		if info.active == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return localGitStopError(errors.New("本地Git进程树停止未确认"))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
