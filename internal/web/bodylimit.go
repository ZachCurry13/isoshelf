package web

import (
	"io"
	"net/http"
)

// maxBody is the most a request may send, other than an upload. The page
// sends a few names, paths or settings at a time; a megabyte is far more than
// any of that, and small enough that a request cannot be used to fill the
// computer's memory, signed in or not.
const maxBody = 1 << 20

// uploadPath takes a whole image. It is written straight to the drive as it
// arrives and is never held in memory, so its size is the drive's business.
const uploadPath = "/api/upload"

// limitBody cuts a request's body off at maxBody. What comes after is never
// read, so a handler sees JSON that ends too soon and refuses it the way it
// refuses any other request it can't understand.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.URL.Path != uploadPath {
			r.Body = cutOff{io.LimitReader(r.Body, maxBody), r.Body}
		}
		next.ServeHTTP(w, r)
	})
}

// cutOff reads up to the limit and closes the real body.
type cutOff struct {
	io.Reader
	io.Closer
}
