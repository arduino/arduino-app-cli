package orchestrator

import (
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// layerProgress keeps track of progress for a single layer.
type layerProgress struct {
	total   uint64
	current uint64
}

// DockerProgressParser parses Docker pull logs to extract and aggregate progress information.
type DockerProgressParser struct {
	mu     sync.Mutex
	layers map[string]*layerProgress

	progressRegex *regexp.Regexp

	history          []uint64 // Memorize the history of progress values
	historySize      int      // the maximum size of the history
	smoothedProgress uint64
}

// NewDockerProgressParser creates a new DockerProgressParser instance.
func NewDockerProgressParser(historySize int) *DockerProgressParser {
	regex := regexp.MustCompile(`^\s*([a-f0-9]{12})\s+(Downloading|Extracting)\s+\[.*\]\s+([\d.]+[kKmMgG]?[bB])\/([\d.]+[kKmMgG]?[bB])`)

	return &DockerProgressParser{
		layers:        make(map[string]*layerProgress),
		progressRegex: regex,
		history:       make([]uint64, 0, historySize),
		historySize:   historySize,
	}
}

// returns the overall progress percentage (0 to 100) and a boolean indicating if parsing was successful.
func (p *DockerProgressParser) Parse(logLine string) (uint64, bool) {

	info, ok := parseProgressLine(logLine, p.progressRegex)
	if !ok {
		return 0, false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.layers[info.layerID]; !exists {
		p.layers[info.layerID] = &layerProgress{}
	}

	p.layers[info.layerID].current = info.currentBytes
	p.layers[info.layerID].total = info.totalBytes

	rawPercentage := calculateTotalProgress(p.layers)

	p.history = append(p.history, rawPercentage)

	// if the history exceeds the maximum size, remove the oldest entry
	if len(p.history) > p.historySize {
		p.history = append(make([]uint64, 0, p.historySize), p.history[1:]...)
	}

	// calculate the smoothed progress as the average of the history
	var sum float64
	for _, v := range p.history {
		sum += float64(v)
	}
	currentSmoothedProgress := sum / float64(len(p.history))
	newSmoothedIntProgress := uint64(currentSmoothedProgress)

	if newSmoothedIntProgress > p.smoothedProgress {
		p.smoothedProgress = newSmoothedIntProgress
		return p.smoothedProgress, true
	}

	return 0, false
}

type parsedProgressInfo struct {
	layerID      string
	currentBytes uint64
	totalBytes   uint64
}

func parseProgressLine(logLine string, regex *regexp.Regexp) (*parsedProgressInfo, bool) {
	matches := regex.FindStringSubmatch(logLine)
	if len(matches) != 5 {
		return nil, false
	}

	currentBytes, err := parseBytes(matches[3])
	if err != nil {
		slog.Warn("Could not retrieve currentBytes from docker progress line", "line", logLine, "error", err)
		return nil, false
	}

	totalBytes, err := parseBytes(matches[4])
	if err != nil {
		slog.Warn("Could not retrieve totalBytes from docker progress line", "line", logLine, "error", err)
		return nil, false
	}

	return &parsedProgressInfo{
		layerID:      matches[1],
		currentBytes: currentBytes,
		totalBytes:   totalBytes,
	}, true
}

func calculateTotalProgress(layers map[string]*layerProgress) uint64 {
	var totalCurrent, grandTotal uint64

	for _, progress := range layers {
		totalCurrent += progress.current
		grandTotal += progress.total
	}

	if grandTotal == 0 {
		return 0
	}

	return uint64((float64(totalCurrent) / float64(grandTotal)) * 100)
}

func parseBytes(s string) (uint64, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	unit := s[len(s)-2:]
	valueStr := s[:len(s)-2]

	var multiplier float64 = 1
	switch unit {
	case "kb":
		multiplier = 1024
	case "mb":
		multiplier = 1024 * 1024
	case "gb":
		multiplier = 1024 * 1024 * 1024
	default:
		unit = s[len(s)-1:]
		valueStr = s[:len(s)-1]
		if unit != "b" {
			valueStr = s
		}
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}

	totalBytesFloat := value * multiplier
	return uint64(totalBytesFloat), nil
}
