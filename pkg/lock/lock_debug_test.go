// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of elf-io

//go:build lockdebug
// +build lockdebug

package lock

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("LockFast", Label("unitest"), func() {
	// it is daemon , add more test here
	It("test debug lock", func() {
		l := &Mutex{}

		l.Lock()
		l.Unlock()
	})
	It("test debug Rlock", func() {
		l := &RWMutex{}
		l.RLock()
		l.RUnlock()
		l.Lock()
		l.Unlock()
	})
})
