// Package clock provides a Clock interface and implementations for time operations.
package clock

import "time"

// Clock is an interface for obtaining the current time.
type Clock interface {
	Now() time.Time
}

// SystemClock is a Clock implementation that delegates to the real system clock.
type SystemClock struct{}

// nowFunc allows tests to override [time.Now] without calling it directly in SystemClock.
var nowFunc = time.Now

// Now returns the current time from nowFunc.
func (SystemClock) Now() time.Time {
	return nowFunc()
}

// Now returns the current time via the package-level SystemClock.
func Now() time.Time {
	return SystemClock{}.Now()
}
