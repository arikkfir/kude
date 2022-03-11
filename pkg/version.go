package pkg

import "github.com/blang/semver"

// TODO: inject version externally (checkout https://blog.alexellis.io/inject-build-time-vars-golang/)

var kudeVersion = semver.MustParse("0.0.1")

func GetVersion() semver.Version {
	return kudeVersion
}
