package version

// Version is the release semver (bump only when cutting a release).
var Version = "0.5.0"

// Commit is injected at build time via -ldflags (short git SHA).
var Commit = "dev"

// String returns a human-readable version line for -version and report footers.
func String() string {
	if Commit != "" && Commit != "dev" {
		return Version + " (" + Commit + ")"
	}
	return Version
}
