# chain

[![test](https://github.com/michaelginalick/aws-sqs-consumer/actions/workflows/test.yml/badge.svg)](https://github.com/michaelginalick/go-chain/actions/workflows/test.yml) 
[![GoDoc](https://godoc.org/github.com/michaelginalick/go-chain?status.svg)](https://godoc.org/github.com/michaelginalick/go-chain)

A generic, type-safe Chain of Responsibility implementation written in Go. Read me about the history of this implementation here: https://michaelginalick.pages.dev/posts/continuing-to-learn/

## Installation

```
go get github.com/michaelginalick/go-chain
```

Requires Go 1.21+.

## Usage

Build a chain from any number of `Rule[T]` implementations and execute them in order.

```go
type Request struct {
    UserID int
    Amount float64
}

c := chain.New(
    chain.NamedFunc("auth", func(ctx context.Context, r Request) error {
        if r.UserID == 0 {
            return errors.New("user not found")
        }
        return nil
    }),
    chain.NamedFunc("amount", func(ctx context.Context, r Request) error {
        if r.Amount <= 0 {
            return errors.New("amount must be positive")
        }
        return nil
    }),
)

if err := c.Execute(ctx, Request{UserID: 1, Amount: 50.0}); err != nil {
    // named rules prefix the error: "auth: user not found"
    log.Fatal(err)
}
```

### Execute vs ExecuteAll

`Execute` stops on the first error. `ExecuteAll` runs every rule and returns all errors joined via `errors.Join`, which is useful for validation where you want every failure at once.

```go
err := c.ExecuteAll(ctx, req)
if errors.Is(err, ErrNoUser) { ... }      // unwraps through the join
if errors.Is(err, ErrBadAmount) { ... }
```

### Implementing Rule[T]

For rules that need state or configuration, implement the interface directly:

```go
type Rule[T any] interface {
    Execute(context.Context, T) error
}
```

Rules can also implement `Namer` to identify themselves in errors:

```go
type rateLimiter struct{ limit int }

func (r *rateLimiter) Name() string { return "rate-limiter" }
func (r *rateLimiter) Execute(ctx context.Context, userID int) error {
    if userID > r.limit {
        return errors.New("limit exceeded")
    }
    return nil
}

c := chain.New[int](&rateLimiter{limit: 1000})
```

### Context

Every rule receives the context passed to `Execute` or `ExecuteAll`. Rules that perform I/O should check `ctx.Err()` before doing work.

```go
chain.NamedFunc("fetch", func(ctx context.Context, r Request) error {
    if err := ctx.Err(); err != nil {
        return err
    }
    // ... do work
    return nil
})
```

## Adapters

| Adapter | Use when |
|---|---|
| `RuleFunc[T]` | Anonymous function, no name needed |
| `NamedFunc(name, fn)` | Inline function with an identity for error context |
| Struct implementing `Rule[T]` | Rule requires state, config, or injected dependencies |
