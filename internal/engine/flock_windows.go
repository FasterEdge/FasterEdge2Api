// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
//go:build windows

package engine

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFileExclusive 对文件句柄加非阻塞排他锁（Windows：LockFileEx）。
func lockFileExclusive(f *os.File) error {
	// 锁整个文件（0..^0），非阻塞 + 排他，语义对应 Unix flock(LOCK_EX|LOCK_NB)。
	// 必须提供有效的 OVERLAPPED 结构:传 nil 会在 Windows 上触发访问冲突。
	var overlapped windows.Overlapped
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, ^uint32(0), ^uint32(0), &overlapped,
	)
}

// unlockFile 释放文件锁。
func unlockFile(f *os.File) {
	var overlapped windows.Overlapped
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, ^uint32(0), ^uint32(0), &overlapped)
}
