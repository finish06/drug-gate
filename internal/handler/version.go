package handler

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/finish06/drug-gate/internal/version"
)

// VersionResponse is the standard build metadata response shared across services.
type VersionResponse struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	GitBranch string `json:"git_branch"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	BuildTime string `json:"build_time"`
}

// VersionInfo handles GET /version.
//
// @Summary      Build version info
// @Description  Returns build metadata: semantic version, git commit hash, git branch, Go runtime version, target OS, target architecture, and build timestamp. All values are injected at compile time via ldflags. Use this endpoint to verify which version of the API is deployed in a given environment. Follows the cross-service version endpoint standard.
// @Tags         system
// @Produce      json
// @Success      200  {object}  VersionResponse
// @Router       /version [get]
func VersionInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(VersionResponse{
		Version:   version.Version,
		GitCommit: version.GitCommit,
		GitBranch: version.GitBranch,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		BuildTime: version.BuildTime,
	})
}
