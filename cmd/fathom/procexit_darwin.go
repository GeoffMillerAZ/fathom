//go:build darwin

package main

import "syscall"

// waitProcessExit blocks until pid exits (kqueue EVFILT_PROC NOTE_EXIT).
// An error means the watch could not be set up (the caller's getppid poll
// still covers it); ESRCH means the process is already gone, which is an
// exit.
func waitProcessExit(pid int) error {
	kq, err := syscall.Kqueue()
	if err != nil {
		return err
	}
	defer syscall.Close(kq)
	change := syscall.Kevent_t{Ident: uint64(pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT}
	events := make([]syscall.Kevent_t, 1)
	for {
		n, err := syscall.Kevent(kq, []syscall.Kevent_t{change}, events, nil)
		if err == syscall.ESRCH {
			return nil
		}
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if n > 0 && events[0].Fflags&syscall.NOTE_EXIT != 0 {
			return nil
		}
		if n > 0 && events[0].Flags&syscall.EV_ERROR != 0 {
			if syscall.Errno(events[0].Data) == syscall.ESRCH {
				return nil
			}
			return syscall.Errno(events[0].Data)
		}
	}
}
