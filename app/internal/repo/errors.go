package repo

import (
	"errors"
	"fmt"
)

func fmtErr(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return errors.New(msg)
}
