package chain_test

import (
	"context"
	"errors"
	"fmt"

	chain "github.com/michaelginalick/go-chain"
)

// Example shows a request-processing pipeline where each named rule identifies
// itself in the returned error, making failures easy to trace.
func Example() {
	type Order struct {
		UserID int
		Amount float64
	}

	var (
		errNoUser   = errors.New("user not found")
		errNegative = errors.New("amount must be positive")
	)

	c := chain.New(
		chain.NamedFunc("auth", func(_ context.Context, o Order) error {
			if o.UserID == 0 {
				return errNoUser
			}
			return nil
		}),
		chain.NamedFunc("amount", func(_ context.Context, o Order) error {
			if o.Amount <= 0 {
				return errNegative
			}
			return nil
		}),
	)

	ctx := context.Background()
	fmt.Println(c.Execute(ctx, Order{UserID: 1, Amount: 50.0}))
	fmt.Println(c.Execute(ctx, Order{UserID: 0, Amount: 50.0}))

	// Output:
	// <nil>
	// auth: user not found
}

// ExampleChain_ExecuteAll shows how to collect every failure in a single pass,
// which is useful for form or payload validation where callers want all errors
// at once rather than fixing one at a time.
func ExampleChain_ExecuteAll() {
	type Registration struct {
		Username string
		Email    string
	}

	var (
		errNoUsername = errors.New("username is required")
		errNoEmail    = errors.New("email is required")
	)

	c := chain.New(
		chain.NamedFunc("username", func(_ context.Context, r Registration) error {
			if r.Username == "" {
				return errNoUsername
			}
			return nil
		}),
		chain.NamedFunc("email", func(_ context.Context, r Registration) error {
			if r.Email == "" {
				return errNoEmail
			}
			return nil
		}),
	)

	err := c.ExecuteAll(context.Background(), Registration{})
	fmt.Println(err)
	fmt.Println(errors.Is(err, errNoUsername))
	fmt.Println(errors.Is(err, errNoEmail))

	// Output:
	// username: username is required
	// email: email is required
	// true
	// true
}

// ExampleRuleFunc shows anonymous function rules. Without a name, errors are
// returned as-is with no wrapping.
func ExampleRuleFunc() {
	sentinel := errors.New("blocked")
	c := chain.New(
		chain.RuleFunc[int](func(_ context.Context, n int) error {
			if n < 0 {
				return sentinel
			}
			return nil
		}),
	)

	ctx := context.Background()
	fmt.Println(c.Execute(ctx, 1))
	err := c.Execute(ctx, -1)
	fmt.Println(err)
	fmt.Println(errors.Is(err, sentinel))

	// Output:
	// <nil>
	// blocked
	// true
}

// ExampleNamer shows a struct rule that implements Namer to attach an identity.
// The name appears as a prefix on any error the rule returns.
func ExampleNamer() {
	c := chain.New(&currencyRule{supported: map[string]bool{"USD": true, "EUR": true}})

	ctx := context.Background()
	fmt.Println(c.Execute(ctx, "USD"))
	fmt.Println(c.Execute(ctx, "XYZ"))

	// Output:
	// <nil>
	// currency: unsupported currency XYZ
}

type currencyRule struct {
	supported map[string]bool
}

func (r *currencyRule) Name() string { return "currency" }
func (r *currencyRule) Execute(_ context.Context, currency string) error {
	if !r.supported[currency] {
		return fmt.Errorf("unsupported currency %s", currency)
	}
	return nil
}

// ExampleChain_Execute_cancellation shows how rules can inspect the context to
// honour cancellation and deadlines. Both NamedFunc and struct-based rules
// receive the context; checking ctx.Err() before doing work is the standard pattern.
func ExampleChain_Execute_cancellation() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the chain runs

	c := chain.New(
		chain.NamedFunc("validate", func(ctx context.Context, _ int) error {
			return ctx.Err()
		}),
		chain.NamedFunc("fetch", func(ctx context.Context, _ int) error {
			return ctx.Err()
		}),
	)

	err := c.Execute(ctx, 0)
	fmt.Println(err)
	fmt.Println(errors.Is(err, context.Canceled))

	// Output:
	// validate: context canceled
	// true
}
