package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	SpinnerStyle     = lipgloss.NewStyle().Foreground(ColorPurple)
	SpinnerTextStyle = lipgloss.NewStyle().Foreground(ColorGray)
)

func NewSpinnerModel() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle
	return s
}

func SpinnerView(message string, s spinner.Model) string {
	if message == "" {
		return s.View()
	}
	return fmt.Sprintf("%s %s", s.View(), SpinnerTextStyle.Render(message))
}

type ConsoleSpinner struct {
	message   string
	frames    []string
	interval  time.Duration
	out       io.Writer
	stop      chan struct{}
	done      chan struct{}
	mu        sync.Mutex
	running   bool
	lastWidth int
}

func NewConsoleSpinner(message string) *ConsoleSpinner {
	sp := spinner.Dot
	return &ConsoleSpinner{
		message:  message,
		frames:   sp.Frames,
		interval: sp.FPS,
		out:      os.Stderr,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (s *ConsoleSpinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.loop()
}

func (s *ConsoleSpinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stop)
	s.mu.Unlock()
	<-s.done
}

func (s *ConsoleSpinner) UpdateMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

func (s *ConsoleSpinner) loop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	idx := 0
	for {
		select {
		case <-s.stop:
			s.clearLine()
			close(s.done)
			return
		case <-ticker.C:
			frame := s.frames[idx%len(s.frames)]
			s.render(frame)
			idx++
		}
	}
}

func (s *ConsoleSpinner) render(frame string) {
	s.mu.Lock()
	msg := s.message
	lineWidth := lipgloss.Width(frame + " " + msg)
	s.lastWidth = lineWidth
	s.mu.Unlock()

	line := SpinnerStyle.Render(frame) + " " + SpinnerTextStyle.Render(msg)
	fmt.Fprint(s.out, "\r"+line)
}

func (s *ConsoleSpinner) clearLine() {
	s.mu.Lock()
	width := s.lastWidth
	s.mu.Unlock()
	if width <= 0 {
		return
	}
	fmt.Fprint(s.out, "\r"+strings.Repeat(" ", width)+"\r")
}
