package version

// Version is set at build time via -ldflags "-X github.com/finish06/drug-gate/internal/version.Version={tag}".
// Defaults to "dev" for local builds.
var Version = "dev"
