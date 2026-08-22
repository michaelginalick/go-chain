// Package chain implements a generic, type-safe Chain of Responsibility.
//
// Build a [Chain] from any number of [Rule] implementations using [New], then
// call [Chain.Execute] to run them in order (stopping on the first error) or
// [Chain.ExecuteAll] to run every rule and collect all errors.
//
// Rules receive a [context.Context] on every call. Checking ctx.Err() before
// performing I/O or other cancellable work is the rule's responsibility.
//
// Rules may optionally implement [Namer] to attach a name to themselves.
// When a named rule returns an error, the chain wraps it as "name: err",
// preserving unwrappability via errors.Is and errors.As.
package chain

import (
	"context"
	"errors"
	"fmt"
)

// Rule is the interface callers implement to participate in a chain.
// Returning a non-nil error halts the chain; returning nil continues to the next rule.
type Rule[T any] interface {
	Execute(context.Context, T) error
}

// Namer is an optional interface a Rule can implement to identify itself.
// When present, errors returned by the rule are wrapped with the name for context.
type Namer interface {
	Name() string
}

// RuleFunc is a function adapter that implements Rule[T].
type RuleFunc[T any] func(context.Context, T) error

func (f RuleFunc[T]) Execute(ctx context.Context, args T) error { return f(ctx, args) }

// namedFunc is a function adapter that implements both Rule[T] and Namer.
type namedFunc[T any] struct {
	name string
	fn   func(context.Context, T) error
}

func (n namedFunc[T]) Name() string                              { return n.name }
func (n namedFunc[T]) Execute(ctx context.Context, args T) error { return n.fn(ctx, args) }

// NamedFunc wraps a function as a Rule[T] that identifies itself by name.
// Errors returned by fn are wrapped with the name, while remaining unwrappable via errors.Is/As.
func NamedFunc[T any](name string, fn func(context.Context, T) error) Rule[T] {
	return namedFunc[T]{name: name, fn: fn}
}

// Chain executes a sequence of Rules in order, stopping on the first error.
type Chain[T any] struct {
	head *node[T]
	tail *node[T]
}

type node[T any] struct {
	name string
	rule Rule[T]
	next *node[T]
}

// New creates a Chain with the provided Rules appended in order.
func New[T any](rules ...Rule[T]) *Chain[T] {
	c := &Chain[T]{}
	c.add(rules...)
	return c
}

// add appends one or more Rules to the end of the chain.
func (c *Chain[T]) add(rules ...Rule[T]) {
	for _, r := range rules {
		if r == nil {
			continue
		}
		n := &node[T]{rule: r}
		if named, ok := r.(Namer); ok {
			n.name = named.Name()
		}
		if c.head == nil {
			c.head = n
			c.tail = n
		} else {
			c.tail.next = n
			c.tail = n
		}
	}
}

// Execute runs each Rule in order. It stops and returns the first error encountered.
func (c *Chain[T]) Execute(ctx context.Context, args T) error {
	cur := c.head
	for cur != nil {
		if err := cur.rule.Execute(ctx, args); err != nil {
			if cur.name != "" {
				return fmt.Errorf("%s: %w", cur.name, err)
			}
			return err
		}
		cur = cur.next
	}
	return nil
}

// ExecuteAll runs each Rule in order. It runs all rules and collects any errors.
func (c *Chain[T]) ExecuteAll(ctx context.Context, args T) error {
	var errs []error
	cur := c.head
	for cur != nil {
		if err := cur.rule.Execute(ctx, args); err != nil {
			if cur.name != "" {
				errs = append(errs, fmt.Errorf("%s: %w", cur.name, err))
			} else {
				errs = append(errs, err)
			}
		}
		cur = cur.next
	}
	return errors.Join(errs...)
}
