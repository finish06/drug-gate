package version

// These variables are set at build time via -ldflags.
// Defaults are used for local builds without ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	GitBranch = "unknown"
	BuildTime = "unknown"
)
