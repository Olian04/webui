package ir

import "time"

// Observer is told what the runtime does, so that the program can count it. It has the methods of the public Observer, which the app's is given to the runtime as.
type Observer interface {
	Begin() func(page, method string, status int, started time.Time)
	Load(page, kind string, started time.Time, err error)
	Action(page, result string)
	Export(page, format string)
	Search(started time.Time)
	SearchFailed(page string)
	NavFailed(page string)
}
