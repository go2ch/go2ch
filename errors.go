package go2ch

var (
	StatusThreadDatOut        = &ThreadError{"thread dat-out", 302}
	StatusNotFound            = &ThreadError{"not found", 302}
	StatusInvalidRangeRequest = &ThreadError{"invalid range request", 416}
	StatusUnknownError        = &ThreadError{"unknown error", 500}
)

type ThreadError struct {
	message    string
	StatusCode int
}

func (e *ThreadError) Error() string {
	return e.message
}
