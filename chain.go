// Package alice provides a convenient way to chain http handlers, together with contexts.
// Modified to no longer only chain handlers, but also pass the contexts like
// suggested by google : https://blog.golang.org/context
package alice

import (
	"net/http"

	"code.google.com/p/go.net/context"
)

// A constructor for a piece of middleware, also for the final method called by .Then
type Constructor func(context.Context, CtxHandler) CtxHandler
type CtxHandlerFunc func(context.Context, http.ResponseWriter, *http.Request)
type CtxHandler interface {
	ServeHTTP(context.Context, http.ResponseWriter, *http.Request)
}

// Chain acts as a list of http.Handler constructors.
// Chain is effectively immutable:
// once created, it will always hold
// the same set of constructors in the same order.
type Chain struct {
	constructors []Constructor
}

// New creates a new chain,
// memorizing the given list of middleware constructors.
// New serves no other function,
// constructors are only called upon a call to Then().
func New(constructors ...Constructor) Chain {
	c := Chain{}
	c.constructors = append(c.constructors, constructors...)

	return c
}

// Then chains the middleware and returns the final http.Handler.
//     New(m1, m2, m3).Then(h)
// is equivalent to:
//     m1(m2(m3(h)))
// When the request comes in, it will be passed to m1, then m2, then m3
// and finally, the given handler
// (assuming every middleware calls the following one).
//
// A chain can be safely reused by calling Then() several times.
//     stdStack := alice.New(ratelimitHandler, csrfHandler)
//     indexPipe = stdStack.Then(indexHandler)
//     authPipe = stdStack.Then(authHandler)
// Note that constructors are called on every call to Then()
// and thus several instances of the same middleware will be created
// when a chain is reused in this way.
// For proper middleware, this should cause no problems.
//
// nil is not allowed for Then()
func (c Chain) Then(h CtxHandler) (wrappedFinal http.Handler) {
	var final CtxHandler

	ctx := context.TODO()

	if h != nil {
		final = h
	} else {
		panic("nil is not allowed")
	}

	for i := len(c.constructors) - 1; i >= 0; i-- {
		final = c.constructors[i](ctx, final)
	}
	wrappedFinal = http.HandlerFunc(CtxHandlerToHandlerFunc(ctx, final))
	return
}

// Same as Then, but with CtxHandler instead of wrapped-http-Handler
func (c Chain) ThenContext(h CtxHandler) (final CtxHandler) {

	ctx := context.TODO()

	if h != nil {
		final = h
	} else {
		panic("nil is not allowed")
	}

	for i := len(c.constructors) - 1; i >= 0; i-- {
		final = c.constructors[i](ctx, final)
	}
	return
}

// Same as ThenFunc, but with CtxHandler instead of wrapped-http-Handler
func (c Chain) ThenFuncContext(fn CtxHandlerFunc) (final CtxHandler) {

	if fn == nil {
		return c.Then(nil)
	}

	return c.ThenContext(CtxHandlerFunc(fn))
}

func CtxHandlerToHandlerFunc(ctx context.Context, fn CtxHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { fn.ServeHTTP(ctx, w, r) }
}

// ThenFunc works identically to Then, but takes
// a HandlerFunc instead of a Handler.
//
// The following two statements are equivalent:
//     c.Then(http.HandlerFunc(fn))
//     c.ThenFunc(fn)
//
// ThenFunc provides all the guarantees of Then.
func (c Chain) ThenFunc(fn CtxHandlerFunc) http.Handler {
	if fn == nil {
		return c.Then(nil)
	}

	return c.Then(CtxHandlerFunc(fn))
}

// Append extends a chain, adding the specified constructors
// as the last ones in the request flow.
//
// Append returns a new chain, leaving the original one untouched.
//
//     stdChain := alice.New(m1, m2)
//     extChain := stdChain.Append(m3, m4)
//     // requests in stdChain go m1 -> m2
//     // requests in extChain go m1 -> m2 -> m3 -> m4
func (c Chain) Append(constructors ...Constructor) Chain {
	newCons := make([]Constructor, len(c.constructors))
	copy(newCons, c.constructors)
	newCons = append(newCons, constructors...)

	newChain := New(newCons...)
	return newChain
}

// ServeHTTP calls f(ctx,w, r).
func (f CtxHandlerFunc) ServeHTTP(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	f(ctx, w, r)
}
