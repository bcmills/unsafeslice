// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package eventually enables the use of finalizers whose registration can be
// blocked until an arbitrary point in the program.
package eventually

import (
	"runtime"
)

var unblocked = make(chan struct{})

func init() {
	close(unblocked)
}

// Block delays finalizer registration until unblock is called.
func Block() (unblock func()) {
	c := make(chan struct{})
	unblocked = c
	return func() { close(c) }
}

// SetFinalizer sets a finalizer f for pointer p.
//
// If registration is currently blocked, SetFinalizer registers it in a
// background goroutine that first waits for registration to be unblocked.
func SetFinalizer(p, f interface{}) {
	select {
	case <-unblocked:
		runtime.SetFinalizer(p, f)
	default:
		go func() {
			<-unblocked
			runtime.SetFinalizer(p, f)
		}()
	}
}
