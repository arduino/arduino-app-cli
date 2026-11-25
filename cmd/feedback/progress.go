// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package feedback

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// Progress shows the progress of an operation: a live row at the bottom of a text terminal,
// with the messages above it. Without a text terminal there is only the messages.
type Progress struct {
	mu      sync.Mutex
	program *tea.Program // nil without a text terminal, and after Done
	done    chan struct{}
	logOut  io.Writer // the output of the log package before the progress, nil if kept
	step    string
	header  string // the last step printed as a header
}

// NewProgress shows a bar for the whole operation. Call Done before the result or an error
// is printed.
func NewProgress() *Progress {
	bar := progress.New(progress.WithColors(tealColor), progress.WithFillCharacters('█', '░'), progress.WithoutPercentage())
	return startProgress(liveRow{bar: &bar})
}

// Spin shows a spinner and text, for a wait with nothing to measure, until stop is called.
func Spin(text string) (stop func()) {
	return startProgress(liveRow{text: text}).Done
}

// Update sets the percent of the whole operation and the step that runs now.
func (p *Progress) Update(step string, percent float32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.program == nil {
		return
	}
	if step != "" {
		p.step = step
	}
	p.program.Send(percentMsg(percent / 100))
}

// Println prints a message. On a terminal it is indented under its step, which prints as a
// header before its first message; a message equal to the step is skipped.
func (p *Progress) Println(line string) {
	p.print(line, lipgloss.NewStyle())
}

// Detail prints a line of the output of a tool as Println does, in gray.
func (p *Progress) Detail(line string) {
	p.print(line, lipgloss.NewStyle().Faint(true))
}

// Done removes the live row.
func (p *Progress) Done() {
	// The log output goes back before mu is taken: the log package holds its lock while it writes.
	if p.logOut != nil {
		log.SetOutput(p.logOut)
	}
	p.mu.Lock()
	program := p.program
	p.program = nil
	p.mu.Unlock()
	if program == nil {
		return
	}
	program.Send(stopMsg{})
	<-p.done
}

// startProgress shows the live row on a text terminal; without one it shows nothing.
func startProgress(row liveRow) *Progress {
	p := &Progress{}
	if format != Text || !isTerminal(stdOut) || !isTerminal(stdErr) {
		return p
	}
	row.spinner = spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(tealColor)))
	row.start = time.Now()
	p.startProgram(row)
	p.routeLog()
	p.stopOnInterrupt()
	return p
}

// startProgram runs the live row. The program reads the terminal answers to its queries;
// once it ends, it takes no more lines.
func (p *Progress) startProgram(row liveRow) {
	var input io.Reader
	if isTerminal(GetStdin()) {
		input = GetStdin()
	}
	program := tea.NewProgram(row, tea.WithInput(input), tea.WithOutput(stdErr), tea.WithoutSignalHandler())
	p.program, p.done = program, make(chan struct{})
	go func() {
		_, _ = program.Run()
		p.mu.Lock()
		p.program = nil
		p.mu.Unlock()
		close(p.done)
	}()
}

// routeLog prints the lines slog writes to the terminal above the live row, until Done.
func (p *Progress) routeLog() {
	if log.Writer() != stdErr {
		return
	}
	p.logOut = log.Writer()
	log.SetOutput(writerFunc(func(b []byte) (int, error) {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.program == nil {
			return p.logOut.Write(b)
		}
		p.program.Println(strings.TrimSuffix(string(b), "\n"))
		return len(b), nil
	}))
}

// stopOnInterrupt calls Done on an interrupt, before the command stops. With the terminal
// read by the program, ctrl+c reaches here as the SIGINT that liveRow sends.
func (p *Progress) stopOnInterrupt() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(interrupt)
		select {
		case <-interrupt:
			p.Done()
		case <-p.done:
		}
	}()
}

// print writes a line for Println and Detail in style.
func (p *Progress) print(line string, style lipgloss.Style) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.program == nil {
		fmt.Fprintln(feedbackOut, line)
		return
	}
	if p.step == "" {
		p.program.Println(style.Render(line))
		return
	}
	if p.step != p.header {
		p.header = p.step
		p.program.Println(tealStyle.Render("▸ " + p.step))
	}
	if line != p.step {
		p.program.Println("  " + style.Render(line))
	}
}

// percentMsg moves the bar to a fraction of the whole operation.
type percentMsg float64

// stopMsg clears the live row and ends its program.
type stopMsg struct{}

// liveRow is the bottom row: a spinner with the text, or a spinner with the bar.
type liveRow struct {
	spinner spinner.Model
	bar     *progress.Model // nil for a spinner with text
	text    string
	percent float64
	width   int
	start   time.Time
	stopped bool
}

func (r liveRow) Init() tea.Cmd { return r.spinner.Tick }

func (r liveRow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
	case percentMsg:
		r.percent = min(max(float64(msg), 0), 1)
	case stopMsg:
		r.stopped = true
		return r, tea.Quit
	case tea.KeyPressMsg:
		if self, err := os.FindProcess(os.Getpid()); err == nil && msg.String() == "ctrl+c" {
			_ = self.Signal(os.Interrupt)
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		r.spinner, cmd = r.spinner.Update(msg)
		return r, cmd
	}
	return r, nil
}

func (r liveRow) View() tea.View {
	if r.stopped {
		return tea.NewView("")
	}
	if r.bar == nil {
		return tea.NewView(r.spinner.View() + " " + r.text)
	}
	label := lipgloss.NewStyle().Bold(true).Render("Progress:")
	percent := fmt.Sprintf("%3.0f%%", r.percent*100)
	took := time.Since(r.start).Truncate(time.Second).String()
	// The bar takes the width the other four parts and the four spaces between them leave.
	bar := *r.bar
	bar.SetWidth(max(r.width-1-len("Progress:")-len(percent)-len(took)-4, 0))
	return tea.NewView(strings.Join([]string{r.spinner.View(), label, percent, bar.ViewAs(r.percent), took}, " "))
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) { return f(b) }
