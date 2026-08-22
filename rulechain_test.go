package chain

import (
	"context"
	"errors"
	"testing"
)

func ctx() context.Context {
	return context.Background()
}

func TestChain_ExecutesAllRulesOnSuccess(t *testing.T) {
	var called []int
	c := New(
		RuleFunc[int](func(_ context.Context, n int) error { called = append(called, 1); return nil }),
		RuleFunc[int](func(_ context.Context, n int) error { called = append(called, 2); return nil }),
		RuleFunc[int](func(_ context.Context, n int) error { called = append(called, 3); return nil }),
	)
	if err := c.Execute(ctx(), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(called) != 3 {
		t.Fatalf("expected 3 rules called, got %d", len(called))
	}
	for i, v := range called {
		if v != i+1 {
			t.Fatalf("expected order [1 2 3], got %v", called)
		}
	}
}

func TestChain_StopsOnFirstError(t *testing.T) {
	sentinel := errors.New("stop here")
	var called []int
	c := New(
		RuleFunc[int](func(_ context.Context, n int) error { called = append(called, 1); return nil }),
		RuleFunc[int](func(_ context.Context, n int) error { called = append(called, 2); return sentinel }),
		RuleFunc[int](func(_ context.Context, n int) error { called = append(called, 3); return nil }),
	)
	err := c.Execute(ctx(), 0)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if len(called) != 2 {
		t.Fatalf("expected chain to stop after rule 2, but %d rules ran", len(called))
	}
}

func TestChain_EmptyChain(t *testing.T) {
	c := New[int]()
	if err := c.Execute(ctx(), 42); err != nil {
		t.Fatalf("empty chain should return nil, got %v", err)
	}
}

func TestChain_AddAppendsInOrder(t *testing.T) {
	var order []int
	c := New(
		RuleFunc[struct{}](func(_ context.Context, _ struct{}) error { order = append(order, 1); return nil }),
		RuleFunc[struct{}](func(_ context.Context, _ struct{}) error { order = append(order, 2); return nil }),
		RuleFunc[struct{}](func(_ context.Context, _ struct{}) error { order = append(order, 3); return nil }),
	)
	_ = c.Execute(ctx(), struct{}{})
	if len(order) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(order))
	}
	for i, v := range order {
		if v != i+1 {
			t.Fatalf("expected order [1 2 3], got %v", order)
		}
	}
}

func TestChain_PassesArgsToAllRules(t *testing.T) {
	type request struct{ value int }
	var seen []int
	c := New(
		RuleFunc[request](func(_ context.Context, r request) error { seen = append(seen, r.value); return nil }),
		RuleFunc[request](func(_ context.Context, r request) error { seen = append(seen, r.value*2); return nil }),
	)
	_ = c.Execute(ctx(), request{value: 7})
	if len(seen) != 2 || seen[0] != 7 || seen[1] != 14 {
		t.Fatalf("unexpected seen values: %v", seen)
	}
}

func TestChain_MutatesPointerArgs(t *testing.T) {
	type state struct{ count int }
	s := &state{}
	c := New(
		RuleFunc[*state](func(_ context.Context, p *state) error { p.count++; return nil }),
		RuleFunc[*state](func(_ context.Context, p *state) error { p.count++; return nil }),
	)
	_ = c.Execute(ctx(), s)
	if s.count != 2 {
		t.Fatalf("expected count 2, got %d", s.count)
	}
}

type authRequest struct{ allowed bool }

var errUnauthorized = errors.New("unauthorized")

type authRule struct{}

func (a *authRule) Execute(_ context.Context, r authRequest) error {
	if !r.allowed {
		return errUnauthorized
	}
	return nil
}

func TestChain_ImplementsRuleInterface(t *testing.T) {
	c := New(
		&authRule{},
		RuleFunc[authRequest](func(_ context.Context, r authRequest) error { return nil }),
	)
	if err := c.Execute(ctx(), authRequest{allowed: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := c.Execute(ctx(), authRequest{allowed: false}); !errors.Is(err, errUnauthorized) {
		t.Fatalf("expected errUnauthorized, got %v", err)
	}
}

func TestChain_NamedFunc_WrapsErrorWithName(t *testing.T) {
	sentinel := errors.New("denied")
	c := New(
		NamedFunc("validate", func(_ context.Context, n int) error { return sentinel }),
	)
	err := c.Execute(ctx(), 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("errors.Is should still find sentinel through wrap, got %v", err)
	}
	if err.Error() != "validate: denied" {
		t.Fatalf("expected wrapped message, got %q", err.Error())
	}
}

func TestChain_UnnamedRule_DoesNotWrapError(t *testing.T) {
	sentinel := errors.New("bare error")
	c := New(RuleFunc[int](func(_ context.Context, n int) error { return sentinel }))
	err := c.Execute(ctx(), 0)
	if err != sentinel {
		t.Fatalf("expected sentinel directly, got %v", err)
	}
}

type namedAuthRule struct{}

func (r *namedAuthRule) Name() string { return "auth" }
func (r *namedAuthRule) Execute(_ context.Context, req authRequest) error {
	if !req.allowed {
		return errUnauthorized
	}
	return nil
}

func TestChain_NamedStruct_WrapsErrorWithName(t *testing.T) {
	c := New(&namedAuthRule{})
	err := c.Execute(ctx(), authRequest{allowed: false})
	if !errors.Is(err, errUnauthorized) {
		t.Fatalf("errors.Is should find errUnauthorized through wrap, got %v", err)
	}
	if err.Error() != "auth: unauthorized" {
		t.Fatalf("expected \"auth: unauthorized\", got %q", err.Error())
	}
}

func TestChain_ExecuteAll_CollectsNamedErrors(t *testing.T) {
	errA := errors.New("problem a")
	errB := errors.New("problem b")
	c := New(
		NamedFunc("step-a", func(_ context.Context, n int) error { return errA }),
		NamedFunc("step-b", func(_ context.Context, n int) error { return errB }),
	)
	err := c.ExecuteAll(ctx(), 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errA) {
		t.Errorf("expected errA in joined error, got %v", err)
	}
	if !errors.Is(err, errB) {
		t.Errorf("expected errB in joined error, got %v", err)
	}
}

func TestChain_NamedFunc_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New(
		NamedFunc("step", func(ctx context.Context, _ int) error {
			return ctx.Err()
		}),
	)
	err := c.Execute(ctx, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
