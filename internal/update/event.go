package update

// EventType defines the type of upgrade event.
type EventType int

const (
	UpgradeLineEvent EventType = iota
	StartEvent
	RestartEvent
	DoneEvent
	ErrorEvent
)

// Event represents a single event in the upgrade process.
type Event struct {
	Type EventType
	Data string
	Err  error // Optional error field for error events
}

func (t EventType) String() string {
	switch t {
	case UpgradeLineEvent:
		return "log"
	case RestartEvent:
		return "restarting"
	case StartEvent:
		return "starting"
	case DoneEvent:
		return "done"
	case ErrorEvent:
		return "error"
	default:
		panic("unreachable")
	}
}

type PackageType string

const (
	Arduino PackageType = "arduino-platform"
	Debian  PackageType = "debian-package"
)

func (s PackageType) AllowedStatuses() []PackageType {
	return []PackageType{Arduino, Debian}
}
