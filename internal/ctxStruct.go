package internal

import "context"

// ContextStruct is struct of ctx
type ContextStruct struct {
	Tag string
	Ctx context.Context
	Cancel context.CancelFunc
}

// NewContextStruct return ContextStruct
func NewContextStruct(c context.Context, cc context.CancelFunc, tag string) *ContextStruct {
	return &ContextStruct{
		Tag:    tag,
		Ctx:    c,
		Cancel: cc,
	}
}