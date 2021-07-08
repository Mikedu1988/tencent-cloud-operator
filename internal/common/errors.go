package common

//ReferencedResourceNotReady error when resource ref is not ready
type ReferencedResourceNotReady struct {
	Message string
}

//Error return error message
func (e *ReferencedResourceNotReady) Error() string {
	return e.Message
}
