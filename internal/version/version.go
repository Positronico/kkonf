package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
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
	version := Version
	gitCommit := GitCommit
	buildDate := BuildDate

	// If version wasn't set via ldflags, try to get it from module info
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			// Check if this is a tagged version from go install
			if info.Main.Version != "(devel)" && info.Main.Version != "" {
				version = info.Main.Version
			}

			// Try to get commit and build info from VCS
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					if gitCommit == "unknown" && len(setting.Value) >= 8 {
						gitCommit = setting.Value[:8]
					}
				case "vcs.time":
					if buildDate == "unknown" {
						buildDate = setting.Value
					}
				}
			}
		}
	}

	return Info{
		Version:   version,
		GitCommit: gitCommit,
		BuildDate: buildDate,
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
