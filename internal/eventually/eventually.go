// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package eventually enables the use of finalizers whose registration can be
// blocked until an arbitrary point in the program.
package eventually

import (
	"runtime"
)

var unblocked chan struct{}

// Block delays finalizer registration until unblock is called.
func Block() (unblock func()) {
	if unblocked != nil {
		panic("cannot Block while already blocked")
	}

	unblocked = make(chan struct{})
	return func() {
		c := unblocked
		unblocked = nil
		close(c)
	}
}

// SetFinalizer sets a finalizer f for pointer p.
//
// If registration is currently blocked, SetFinalizer registers it in a
// background goroutine that first waits for registration to be unblocked.
func SetFinalizer(p, f interface{}) {
	if unblocked == nil {
		runtime.SetFinalizer(p, f)
	} else {
		go func(c <-chan struct{}) {
			<-c
			runtime.SetFinalizer(p, f)
		}(unblocked)
	}
}
