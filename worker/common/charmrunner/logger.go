// Copyright 2012-2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmrunner

import (
	"bufio"
	"io"
	"sync"
	"time"

	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("juju.worker.common.runner")

// MessageReceiver instances are fed messages written to stdout/stderr
// when running hooks/actions.
type MessageReceiver interface {
	Messagef(isPrefix bool, message string, args ...interface{})
}

// NewHookLogger creates a new hook logger.
func NewHookLogger(outReader io.ReadCloser, receivers ...MessageReceiver) *HookLogger {
	return &HookLogger{
		r:         outReader,
		done:      make(chan struct{}),
		receivers: receivers,
	}
}

// HookLogger streams the output from a hook to message receivers.
type HookLogger struct {
	r         io.ReadCloser
	done      chan struct{}
	mu        sync.Mutex
	stopped   bool
	receivers []MessageReceiver
}

// Run starts the hook logger.
func (l *HookLogger) Run() {
	defer close(l.done)
	defer l.r.Close()
	br := bufio.NewReaderSize(l.r, 4096)
	for {
		line, isPrefix, err := br.ReadLine()
		if err != nil {
			if err != io.EOF {
				logger.Errorf("cannot read hook output: %v", err)
			}
			break
		}
		l.mu.Lock()
		if l.stopped {
			l.mu.Unlock()
			return
		}
		for _, r := range l.receivers {
			r.Messagef(isPrefix, "%s", line)
		}
		l.mu.Unlock()
	}
}

// Stop stops the hook logger.
func (l *HookLogger) Stop() {
	// We can see the process exit before the logger has processed
	// all its output, so allow a moment for the data buffered
	// in the pipe to be processed. We don't wait indefinitely though,
	// because the hook may have started a background process
	// that keeps the pipe open.
	select {
	case <-l.done:
	case <-time.After(100 * time.Millisecond):
	}
	// We can't close the pipe asynchronously, so just
	// stifle output instead.
	l.mu.Lock()
	l.stopped = true
	l.mu.Unlock()
}
