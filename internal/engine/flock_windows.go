//go:build windows

package engine

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFileExclusive 对文件句柄加非阻塞排他锁（Windows：LockFileEx）。
func lockFileExclusive(f *os.File) error {
	// 锁整个文件（0..^0），非阻塞 + 排他，语义对应 Unix flock(LOCK_EX|LOCK_NB)。
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, ^uint32(0), ^uint32(0), nil,
	)
}

// unlockFile 释放文件锁。
func unlockFile(f *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, ^uint32(0), ^uint32(0), nil)
}