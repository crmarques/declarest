package managedserver

import (
	"errors"
	"fmt"
	"net/http"
)

type HTTPError struct {
	Method     string
	URL        string
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s %s: unexpected status %d", e.Method, e.URL, e.StatusCode)
}

func (e *HTTPError) Status() int {
	if e == nil {
		return 0
	}
	return e.StatusCode
}

func IsNotFoundError(err error) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusNotFound
	}
	return false
}

func IsConflictError(err error) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusConflict
	}
	return false
}
