package clock

import "time"

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now()
}

func Now() time.Time {
	return SystemClock{}.Now()
}
