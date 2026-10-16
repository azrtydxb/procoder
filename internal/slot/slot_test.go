package slot

import (
	"context"
	"errors"
	"testing"
	"time"
)

// proved by: made tryLock always succeed — the second acquirer gets a slot
// while the first still holds it, and the machine is unbounded again.
func TestASecondAcquirerWaitsThenGivesUp(t *testing.T) {
	if !Supported {
		t.Skip("no slots on this platform")
	}
	dir := t.TempDir()
	release, err := Acquire(context.Background(), dir, "job", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = Acquire(ctx, dir, "job", 1)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("want ErrBusy while the only slot is held, got %v", err)
	}
	if waited := time.Since(start); waited < 500*time.Millisecond {
		t.Errorf("gave up after %s without waiting out its context", waited)
	}
}

// proved by: dropped the unlock from release — the waiter never gets in.
func TestAWaiterGetsTheSlotWhenItIsReleased(t *testing.T) {
	if !Supported {
		t.Skip("no slots on this platform")
	}
	dir := t.TempDir()
	release, err := Acquire(context.Background(), dir, "job", 1)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(300*time.Millisecond, release)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	second, err := Acquire(ctx, dir, "job", 1)
	if err != nil {
		t.Fatalf("the waiter must get the slot once it is free: %v", err)
	}
	second()
}

// n slots admit n holders at once and refuse the n+1th.
// proved by: ignored n and opened only name-0 — the second holder waits.
func TestNSlotsAdmitN(t *testing.T) {
	if !Supported {
		t.Skip("no slots on this platform")
	}
	dir := t.TempDir()
	a, err := Acquire(context.Background(), dir, "job", 2)
	if err != nil {
		t.Fatal(err)
	}
	defer a()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	b, err := Acquire(ctx, dir, "job", 2)
	if err != nil {
		t.Fatalf("the second of two slots must be free: %v", err)
	}
	defer b()
	if _, err := Acquire(ctx, dir, "job", 2); !errors.Is(err, ErrBusy) {
		t.Fatalf("a third holder of two slots must wait and give up, got %v", err)
	}
}
