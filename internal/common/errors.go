package common

type ReferencedResourceNotReady struct {
	Message string
}

func (e *ReferencedResourceNotReady) Error() string {
	return e.Message
}
