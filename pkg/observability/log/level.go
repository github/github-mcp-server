package log

// Level represents the log level, from debug to fatal
type Level struct {
	level string
}

var (
	// DebugLevel causes all logs to be logged
	DebugLevel = Level{"debug"}
	// InfoLevel causes all logs of level info or more severe to be logged
	InfoLevel = Level{"info"}
	// WarnLevel causes all logs of level warn or more severe to be logged
	WarnLevel = Level{"warn"}
	// ErrorLevel causes all logs of level error or more severe to be logged
	ErrorLevel = Level{"error"}
	// FatalLevel causes only logs of level fatal to be logged
	FatalLevel = Level{"fatal"}
)

// String returns the string representation for Level
//
// This is useful when trying to get the string values for Level and mapping level in other external libraries. For example:
// ```
// trace.SetLogLevel(kvp.String("loglevel", log.DebugLevel.String())
// ```
func (l Level) String() string {
	return l.level
}
