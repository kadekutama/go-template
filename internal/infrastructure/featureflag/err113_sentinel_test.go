package featureflag_test

import "errors"

// Static test sentinels for err113 in tests.
var (
	// errCacheMiss marks a stub-cache miss.
	errCacheMiss = errors.New("featureflag test: cache miss")
	// errDialFailed marks a scripted base-transport failure.
	errDialFailed = errors.New("featureflag test: dial failed")
)
