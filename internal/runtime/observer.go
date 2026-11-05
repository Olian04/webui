package runtime

import "time"

// noObserver is the Observer of an app that was given none: it is told, and does nothing.
type noObserver struct{}

func (noObserver) Begin() func(string, string, int, time.Time) {
	return func(string, string, int, time.Time) {}
}
func (noObserver) Load(string, string, time.Time, error) {}
func (noObserver) Action(string, string)                 {}
func (noObserver) Export(string, string)                 {}
func (noObserver) Search(time.Time)                      {}
func (noObserver) SearchFailed(string)                   {}
func (noObserver) NavFailed(string)                      {}
