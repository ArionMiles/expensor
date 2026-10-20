//go:build unix

package sqlite

import (
	"os"
	"syscall"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int64(stat.Uid) == int64(os.Geteuid())
}

func trustedSystemSymlink(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0
}

func pathFailureKind(err error) errors.Kind {
	switch {
	case os.IsPermission(err):
		return errors.PermissionDenied
	case errors.Is(err, syscall.ENAMETOOLONG), errors.Is(err, syscall.ENOTDIR), errors.Is(err, syscall.ELOOP):
		return errors.InvalidArgument
	default:
		return errors.Internal
	}
}
