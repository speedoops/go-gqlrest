package handlerx

import "context"

type ResponseContext struct {
	context.Context
	total *int64
}

type responseContextType string

var responseContextKey responseContextType = "gogqlrest_response_context"

func GetResponseContext(ctx context.Context) *ResponseContext {
	if v, ok := ctx.Value(responseContextKey).(*ResponseContext); ok {
		return v
	}

	return nil
}

func createResponseContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, responseContextKey, &ResponseContext{Context: ctx})
}

func (c *ResponseContext) Total() *int64 {
	return c.total
}

func (c *ResponseContext) SetTotal(total int64) {
	c.total = &total
}
