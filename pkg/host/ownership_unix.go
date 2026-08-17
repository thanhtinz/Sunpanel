//go:build !windows

package host

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// fillOwnership điền chủ sở hữu và nhóm của tệp trên các hệ Unix.
// Nếu không tra cứu được tên, giữ nguyên UID/GID dạng số.
func fillOwnership(info *FileInfo, st os.FileInfo) {
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}

	uid := strconv.FormatUint(uint64(sys.Uid), 10)
	gid := strconv.FormatUint(uint64(sys.Gid), 10)
	info.Owner, info.Group = uid, gid

	if u, err := user.LookupId(uid); err == nil {
		info.Owner = u.Username
	}
	if g, err := user.LookupGroupId(gid); err == nil {
		info.Group = g.Name
	}
}
