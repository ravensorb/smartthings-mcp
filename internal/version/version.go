package version

// Set via ldflags at build time. Defaults here serve local `go build` without flags.
var (
	Version = "0.0.3"
	Commit  = "unknown"
	Date    = "unknown"
)
