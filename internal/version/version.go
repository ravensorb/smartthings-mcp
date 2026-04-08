package version

// Set via ldflags at build time. Defaults here serve local `go build` without flags.
var (
	Version = "0.1.0"
	Commit  = "unknown"
	Date    = "unknown"
)
