package kude

import "github.com/blang/semver"

var gitCommit = "unknown"
var gitTag = "0.0.0-dev"
var version semver.Version

func init() {
	if gitTag[0] == 'v' {
		gitTag = gitTag[1:]
	}
	version = semver.MustParse(gitTag + "+" + gitCommit)
}

// GetVersion returns the kude version currently running.
func GetVersion() semver.Version {
	return version
}
