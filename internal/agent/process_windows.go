package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

func runCodexProcess(ctx context.Context, cmd *exec.Cmd) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	job, err := codexJob()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	input, err := codexInputFile(cmd.Stdin)
	if err != nil {
		return err
	}
	defer input.Close()
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	handles, err := codexHandles(input, null, os.Stderr)
	if err != nil {
		return err
	}
	defer func() {
		for _, h := range handles {
			_ = windows.CloseHandle(h)
		}
	}()
	process, err := startCodexProcess(cmd, job, handles)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process.Process)
	defer windows.CloseHandle(process.Thread)
	return waitCodexProcess(ctx, job, process.Process)
}

func codexJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func codexInputFile(reader io.Reader) (*os.File, error) {
	name := filepath.Join(os.TempDir(), "pangu-codex-input-"+uuid.NewString()+".json")
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	// 内核在最后一个继承句柄关闭时删除输入，Worker 崩溃也不残留提示词。
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_TEMPORARY|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	if _, err = io.Copy(file, reader); err == nil {
		_, err = file.Seek(0, io.SeekStart)
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func codexHandles(files ...*os.File) ([]windows.Handle, error) {
	var handles []windows.Handle
	for _, file := range files {
		var handle windows.Handle
		err := windows.DuplicateHandle(windows.CurrentProcess(), windows.Handle(file.Fd()),
			windows.CurrentProcess(), &handle, 0, true, windows.DUPLICATE_SAME_ACCESS)
		if err != nil {
			for _, h := range handles {
				_ = windows.CloseHandle(h)
			}
			return nil, err
		}
		handles = append(handles, handle)
	}
	return handles, nil
}

func startCodexProcess(cmd *exec.Cmd, job windows.Handle, handles []windows.Handle) (*windows.ProcessInformation, error) {
	attributes, err := windows.NewProcThreadAttributeList(2)
	if err != nil {
		return nil, err
	}
	defer attributes.Delete()
	// PROC_THREAD_ATTRIBUTE_JOB_LIST：Windows 10+ 在创建时原子入组，不能先运行再加入 Job。
	if err = attributes.Update(0x0002000D, unsafe.Pointer(&job), unsafe.Sizeof(job)); err != nil {
		return nil, err
	}
	if err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]),
		uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{ProcThreadAttributeList: attributes.List()}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.StdInput, startup.StdOutput, startup.StdErr = handles[0], handles[1], handles[2]
	application, err := windows.UTF16PtrFromString(cmd.Path)
	if err != nil {
		return nil, err
	}
	command, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(cmd.Args))
	if err != nil {
		return nil, err
	}
	directory, err := windows.UTF16PtrFromString(cmd.Dir)
	if err != nil {
		return nil, err
	}
	environment := utf16.Encode([]rune(strings.Join(cmd.Env, "\x00") + "\x00\x00"))
	process := &windows.ProcessInformation{}
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NO_WINDOW)
	err = windows.CreateProcess(application, command, nil, nil, true, flags, &environment[0], directory, &startup.StartupInfo, process)
	return process, err
}

func waitCodexProcess(ctx context.Context, job, process windows.Handle) error {
	done := make(chan error, 1)
	go func() { _, err := windows.WaitForSingleObject(process, windows.INFINITE); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		if err := windows.TerminateJobObject(job, 1); err != nil {
			return err
		}
		<-done
		return ctx.Err()
	}
	var code uint32
	if err := windows.GetExitCodeProcess(process, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("进程退出码 %d", code)
	}
	return nil
}
