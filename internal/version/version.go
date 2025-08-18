package version

import (
	"fmt"
	"runtime"
)

var (
	// These variables are set via ldflags during build
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Info holds version information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get returns version information
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string
func (i Info) String() string {
	if i.Version == "dev" {
		return fmt.Sprintf("kkonf %s (commit: %s)", i.Version, i.GitCommit)
	}
	return fmt.Sprintf("kkonf %s", i.Version)
}

// Detailed returns a detailed version string
func (i Info) Detailed() string {
	return fmt.Sprintf(`kkonf version %s
Git commit: %s
Build date: %s
Go version: %s
Platform: %s`, i.Version, i.GitCommit, i.BuildDate, i.GoVersion, i.Platform)
}