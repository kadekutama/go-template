package logging

// DefaultConfig is the adapter baseline; binaries override Level from the
// observability section of Config (E01-T03) once E11 wires it.
var DefaultConfig = Config{Level: "info"}
