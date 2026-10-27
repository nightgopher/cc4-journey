package repo

import (
	"errors"
	"fmt"
)

var ErrForbidden = errors.New("无操作权限")

func fmtErr(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return errors.New(msg)
}
