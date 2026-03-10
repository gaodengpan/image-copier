package progress

import (
	"testing"
	"time"
)

func TestAbortWorkers_AbortsAllBars(t *testing.T) {
	p := NewProgress(1, 1, false, "pulling")

	time.Sleep(100 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		p.AbortWorkers()
		p.WaitContainer()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AbortWorkers/WaitContainer blocked - likely deadlock")
	}
}

func TestWaitContainer_ReturnsAfterAbort(t *testing.T) {
	p := NewProgress(1, 1, false, "pulling")

	p.AbortWorkers()

	done := make(chan struct{})
	go func() {
		p.WaitContainer()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitContainer blocked after AbortWorkers")
	}
}

func TestProgress_NewProgressWithZeroTotal_Abort(t *testing.T) {
	p := NewProgress(0, 1, false, "pulling")

	done := make(chan struct{})
	go func() {
		p.AbortWorkers()
		p.WaitContainer()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AbortWorkers/WaitContainer blocked with zero total")
	}
}
