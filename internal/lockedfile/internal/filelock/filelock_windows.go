// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package filelock

import (
	"github.com/pkg/errors"
)

func lock(f File, lt lockType) error {
	return errors.New("not implemented")

}

func unlock(f File) error {
	return errors.New("not implemented")
}

func isNotSupported(err error) bool {
	return true
}
