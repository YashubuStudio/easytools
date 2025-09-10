package logging

import (
	"strings"
	"sync"
)

type LogRing struct {
	mu  sync.Mutex
	max int
	buf []string
}

func NewLogRing(max int) *LogRing { return &LogRing{max: max, buf: make([]string, 0, max)} }

func (r *LogRing) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, line := range strings.Split(string(p), "\n") {
		if line == "" {
			continue
		}
		if len(r.buf) >= r.max {
			r.buf = r.buf[1:]
		}
		r.buf = append(r.buf, line)
	}
	return len(p), nil
}

func (r *LogRing) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.buf, "\n")
}
