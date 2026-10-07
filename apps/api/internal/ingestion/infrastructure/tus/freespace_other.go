//go:build !unix

package tus

import "math"

// FreeSpace is not measured on non-Unix systems (the app runs on Linux; this
// only keeps the package compiling elsewhere).
func FreeSpace(string) (uint64, error) { return math.MaxUint64, nil }
