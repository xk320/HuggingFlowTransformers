package supervisor

import (
	"context"
	"strings"
	"sync"
	"time"
)

type LineKind int

const (
	LineOther LineKind = iota
	LineModelDataSubmitted
	LineFatalUnsupportedGPU
	LineProcessExit
)

type EngineLine struct {
	Device int
	Text   string
}

type Event struct {
	Device int
	Name   string
	Level  string
}

type EventSink interface {
	Event(Event)
}

type DiscardEvents struct{}

func (DiscardEvents) Event(Event) {}

type Runner interface {
	StartDevice(ctx context.Context, device int, lines chan<- EngineLine) error
	RestartDevice(ctx context.Context, device int, lines chan<- EngineLine) error
	StopDevice(device int) error
}

type Config struct {
	Devices            []int
	NoModelDataTimeout time.Duration
	RestartOnExit      bool
	Runner             Runner
	EventSink          EventSink
}

type Supervisor struct {
	config   Config
	lines    chan EngineLine
	done     chan struct{}
	stop     context.CancelFunc
	tracker  *ModelDataTracker
	disabled map[int]bool
	mu       sync.Mutex
}

func New(config Config) *Supervisor {
	if config.EventSink == nil {
		config.EventSink = DiscardEvents{}
	}
	return &Supervisor{
		config:   config,
		lines:    make(chan EngineLine, 128),
		done:     make(chan struct{}),
		tracker:  NewModelDataTracker(config.NoModelDataTimeout),
		disabled: map[int]bool{},
	}
}

func (s *Supervisor) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.stop = cancel
	now := time.Now()
	for _, device := range s.config.Devices {
		s.tracker.Start(device, now)
		if err := s.config.Runner.StartDevice(ctx, device, s.lines); err != nil {
			cancel()
			return err
		}
	}
	go s.loop(ctx)
	return nil
}

func (s *Supervisor) Wait() {
	<-s.done
}

func (s *Supervisor) loop(ctx context.Context) {
	defer close(s.done)
	interval := s.config.NoModelDataTimeout / 4
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			for _, device := range s.config.Devices {
				_ = s.config.Runner.StopDevice(device)
			}
			return
		case line := <-s.lines:
			s.handleLine(ctx, line)
		case now := <-ticker.C:
			s.checkTimeouts(ctx, now)
		}
	}
}

func (s *Supervisor) handleLine(ctx context.Context, line EngineLine) {
	switch ClassifyEngineLine(line.Text) {
	case LineModelDataSubmitted:
		s.tracker.MarkSubmitted(line.Device, time.Now())
		s.config.EventSink.Event(Event{Device: line.Device, Name: "model_data_submitted", Level: "info"})
	case LineFatalUnsupportedGPU:
		s.config.EventSink.Event(Event{Device: line.Device, Name: "fatal_unsupported_gpu", Level: "error"})
		s.disable(line.Device)
		_ = s.config.Runner.StopDevice(line.Device)
	case LineProcessExit:
		if s.config.RestartOnExit {
			s.config.EventSink.Event(Event{Device: line.Device, Name: "runtime_worker_exit", Level: "warn"})
			s.config.EventSink.Event(Event{Device: line.Device, Name: "runtime_recovered", Level: "warn"})
			_ = s.config.Runner.RestartDevice(ctx, line.Device, s.lines)
			s.tracker.MarkSubmitted(line.Device, time.Now())
		}
	default:
		_ = ctx
	}
}

func (s *Supervisor) checkTimeouts(ctx context.Context, now time.Time) {
	for _, device := range s.config.Devices {
		if s.isDisabled(device) {
			continue
		}
		if !s.tracker.TimedOut(device, now) {
			continue
		}
		s.config.EventSink.Event(Event{Device: device, Name: "model_data_timeout", Level: "warn"})
		_ = s.config.Runner.RestartDevice(ctx, device, s.lines)
		s.tracker.MarkSubmitted(device, now)
		s.config.EventSink.Event(Event{Device: device, Name: "runtime_recovered", Level: "info"})
	}
}

func (s *Supervisor) disable(device int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disabled[device] = true
}

func (s *Supervisor) isDisabled(device int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disabled[device]
}

func ClassifyEngineLine(line string) LineKind {
	line = strings.ToLower(stripANSI(line))
	switch {
	case strings.Contains(line, "no kernel image is available for execution on the device"):
		return LineFatalUnsupportedGPU
	case strings.Contains(line, "runtime_worker_exit") || strings.Contains(line, "hft_process_exit"):
		return LineProcessExit
	case strings.Contains(line, "component=share") && (strings.Contains(line, " submitted") || strings.Contains(line, " accepted")):
		return LineModelDataSubmitted
	case strings.Contains(line, "event=model_data_submitted"):
		return LineModelDataSubmitted
	default:
		return LineOther
	}
}

func stripANSI(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == 0x1b && i+1 < len(value) && value[i+1] == '[' {
			i += 2
			for i < len(value) {
				c := value[i]
				if c >= '@' && c <= '~' {
					break
				}
				i++
			}
			continue
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

type ModelDataTracker struct {
	timeout time.Duration
	mu      sync.Mutex
	latest  map[int]time.Time
}

func NewModelDataTracker(timeout time.Duration) *ModelDataTracker {
	return &ModelDataTracker{timeout: timeout, latest: map[int]time.Time{}}
}

func (t *ModelDataTracker) Start(device int, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.latest[device] = now
}

func (t *ModelDataTracker) MarkSubmitted(device int, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.latest[device] = now
}

func (t *ModelDataTracker) TimedOut(device int, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	latest, ok := t.latest[device]
	return ok && t.timeout > 0 && now.Sub(latest) > t.timeout
}

type FakeRunner struct {
	mu       sync.Mutex
	lines    chan<- EngineLine
	restarts map[int]int
}

func (f *FakeRunner) StartDevice(_ context.Context, _ int, lines chan<- EngineLine) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = lines
	if f.restarts == nil {
		f.restarts = map[int]int{}
	}
	return nil
}

func (f *FakeRunner) RestartDevice(_ context.Context, device int, lines chan<- EngineLine) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = lines
	if f.restarts == nil {
		f.restarts = map[int]int{}
	}
	f.restarts[device]++
	return nil
}

func (f *FakeRunner) StopDevice(_ int) error {
	return nil
}

func (f *FakeRunner) Emit(device int, text string) {
	f.mu.Lock()
	lines := f.lines
	f.mu.Unlock()
	if lines != nil {
		lines <- EngineLine{Device: device, Text: text}
	}
}

func (f *FakeRunner) Restarts(device int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.restarts[device]
}
