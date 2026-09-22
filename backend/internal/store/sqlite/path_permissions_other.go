//go:build !unix

package sqlite

import (
	"os"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func ownedByCurrentUser(os.FileInfo) bool {
	return true
}

func trustedSystemSymlink(os.FileInfo) bool {
	return false
}

func pathFailureKind(err error) errors.Kind {
	if os.IsPermission(err) {
		return errors.PermissionDenied
	}
	return errors.Internal
}
