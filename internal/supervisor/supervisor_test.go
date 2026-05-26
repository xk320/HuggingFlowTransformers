package supervisor

import (
	"context"
	"testing"
	"time"
)

func TestClassifyEngineLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		want LineKind
	}{
		{
			name: "submitted model data",
			line: "2026-05-26T07:09:28Z level=INFO component=share submitted job=abc",
			want: LineModelDataSubmitted,
		},
		{
			name: "fatal unsupported gpu",
			line: `level=WARN component=pool reconnect_failed error="solve_blake3_challenge_kernel: no kernel image is available for execution on the device"`,
			want: LineFatalUnsupportedGPU,
		},
		{
			name: "other",
			line: "ordinary internal line",
			want: LineOther,
		},
		{
			name: "runtime worker exit",
			line: "runtime_worker_exit: signal killed",
			want: LineProcessExit,
		},
		{
			name: "legacy worker exit",
			line: "hft_process_exit: signal killed",
			want: LineProcessExit,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyEngineLine(tt.line); got != tt.want {
				t.Fatalf("ClassifyEngineLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoModelDataTrackerTimesOutPerDevice(t *testing.T) {
	tracker := NewModelDataTracker(300 * time.Second)
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

	tracker.Start(0, now)
	tracker.Start(1, now)
	tracker.MarkSubmitted(1, now.Add(250*time.Second))

	if tracker.TimedOut(0, now.Add(301*time.Second)) != true {
		t.Fatal("device 0 should time out")
	}
	if tracker.TimedOut(1, now.Add(301*time.Second)) != false {
		t.Fatal("device 1 should not time out because it submitted recently")
	}
}

func TestSupervisorRestartsOnlyTimedOutDevice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fake := &FakeRunner{}
	s := New(Config{
		Devices:            []int{0, 1},
		NoModelDataTimeout: 20 * time.Millisecond,
		Runner:             fake,
		EventSink:          DiscardEvents{},
	})

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fake.Emit(1, "component=share submitted")
			}
		}
	}()
	time.Sleep(70 * time.Millisecond)
	cancel()
	s.Wait()

	if fake.Restarts(0) == 0 {
		t.Fatal("expected device 0 to restart")
	}
	if fake.Restarts(1) != 0 {
		t.Fatalf("expected device 1 not to restart, got %d", fake.Restarts(1))
	}
}

func TestFatalUnsupportedGPUDisablesFutureRestarts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fake := &FakeRunner{}
	s := New(Config{
		Devices:            []int{0},
		NoModelDataTimeout: 20 * time.Millisecond,
		RestartOnExit:      true,
		Runner:             fake,
		EventSink:          DiscardEvents{},
	})

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	fake.Emit(0, "no kernel image is available for execution on the device")
	time.Sleep(70 * time.Millisecond)
	cancel()
	s.Wait()

	if fake.Restarts(0) != 0 {
		t.Fatalf("fatal unsupported GPU should not restart, got %d", fake.Restarts(0))
	}
}

func TestProcessExitEmitsRuntimeEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fake := &FakeRunner{}
	sink := &recordingSink{}
	s := New(Config{
		Devices:            []int{0},
		NoModelDataTimeout: time.Hour,
		RestartOnExit:      true,
		Runner:             fake,
		EventSink:          sink,
	})

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	fake.Emit(0, "runtime_worker_exit: signal killed")
	time.Sleep(30 * time.Millisecond)
	cancel()
	s.Wait()

	if fake.Restarts(0) == 0 {
		t.Fatal("expected restart after process exit")
	}
	if !sink.Has("runtime_worker_exit") {
		t.Fatalf("missing runtime_worker_exit event: %#v", sink.events)
	}
	if !sink.Has("runtime_recovered") {
		t.Fatalf("missing runtime_recovered event: %#v", sink.events)
	}
}

type recordingSink struct {
	events []Event
}

func (r *recordingSink) Event(event Event) {
	r.events = append(r.events, event)
}

func (r *recordingSink) Has(name string) bool {
	for _, event := range r.events {
		if event.Name == name {
			return true
		}
	}
	return false
}
