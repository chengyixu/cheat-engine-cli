//go:build !windows && (!darwin || !cgo)

package localbridge

import "errors"

func NewSystemBackend() (Backend, error) {
	return nil, errors.New("native process access requires Windows or a cgo-enabled macOS build")
}
