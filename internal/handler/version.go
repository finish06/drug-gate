package handler

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/finish06/drug-gate/internal/version"
)

// VersionInfo handles GET /version.
//
// @Summary      Build version info
// @Description  Returns build version, git commit, git branch, and Go runtime version.
// @Tags         public
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /version [get]
func VersionInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"version":    version.Version,
		"git_commit": version.GitCommit,
		"git_branch": version.GitBranch,
		"go_version": runtime.Version(),
	})
}
