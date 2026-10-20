package ir

// Refusal is an error that the visitor is told of, in place of "something went wrong": a status, what to say, and where to
// go from there. A Guard that returns one answers with its status, and a panel's Load that returns one says it in the
// panel, since a status cannot change once a page has begun to be sent. Any other error is the app's and is logged.
type Refusal struct {
	Status  int    // 401, 403 or 404
	Message string // what the visitor reads
	Label   string // the words of the link that leads on, "" for none
	Href    string // where it leads, an address of this app
}

func (r *Refusal) Error() string { return r.Message }
