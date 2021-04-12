package controllers

type ReferencedResourceNotReady struct {
	Message string
}

func (e *ReferencedResourceNotReady) Error() string {
	return e.Message
}

type InvalidVpcId struct {
	Message string
}

func (e *InvalidVpcId) Error() string {
	return e.Message
}
