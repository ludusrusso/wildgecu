package daemon

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ludusrusso/wildgecu/pkg/agent"
	"github.com/ludusrusso/wildgecu/pkg/home"
	"github.com/ludusrusso/wildgecu/pkg/provider"
	"github.com/ludusrusso/wildgecu/pkg/session"
)

const (
	concurrencyUserMsg      = "user input"
	concurrencyAssistantMsg = "ok"
)

// gatedProvider blocks Generate until release is closed (or ctx is canceled),
// closing started when it first enters Generate so callers can deterministically
// wait for the turn to be in flight.
type gatedProvider struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGatedProvider() *gatedProvider {
	return &gatedProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (p *gatedProvider) Generate(ctx context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
	p.once.Do(func() { close(p.started) })
	select {
	case <-p.release:
		return &provider.Response{Message: provider.Message{Role: provider.RoleModel, Content: concurrencyAssistantMsg}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// slowProvider returns a canned response after sleeping for delay or until ctx
// is canceled. Used by the stress subtest to model an in-flight turn that
// finishes naturally if no one cancels it.
type slowProvider struct {
	delay time.Duration
}

func (p slowProvider) Generate(ctx context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
	select {
	case <-time.After(p.delay):
		return &provider.Response{Message: provider.Message{Role: provider.RoleModel, Content: concurrencyAssistantMsg}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func newConcurrencyTestSM(t *testing.T, p provider.Provider) *SessionManager {
	t.Helper()
	h, err := home.New(t.TempDir())
	if err != nil {
		t.Fatalf("home.New: %v", err)
	}
	return &SessionManager{
		agentCfg: agent.Config{
			Provider: fakeProvider{}, // used by Finalize; must not block
			Home:     h,
		},
		chatCfg: &session.Config{
			Provider:    p,
			WelcomeText: "hello",
		},
		sessions: make(map[string]*ManagedSession),
	}
}

// startGatedTurn launches RunTurnStream in a goroutine and returns a channel
// receiving the turn's terminal error. Blocks until the gatedProvider has
// entered Generate so the caller can synchronize on an in-flight turn.
func startGatedTurn(sm *SessionManager, p *gatedProvider, id, input string) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := sm.RunTurnStream(context.Background(), id, input, nil, nil, nil, nil)
		done <- err
	}()
	<-p.started
	return done
}

func TestSessionConcurrency(t *testing.T) {
	t.Run("messages snapshot serializes with in-flight turn", func(t *testing.T) {
		p := newGatedProvider()
		sm := newConcurrencyTestSM(t, p)
		sess := sm.Create()

		turnDone := startGatedTurn(sm, p, sess.ID, concurrencyUserMsg)

		msgsCh := make(chan []provider.Message, 1)
		go func() { msgsCh <- sess.Messages() }()

		select {
		case <-msgsCh:
			t.Fatal("Messages() returned while turn was in flight")
		case <-time.After(20 * time.Millisecond):
		}

		close(p.release)

		select {
		case err := <-turnDone:
			if err != nil {
				t.Fatalf("turn returned error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("turn did not return after release")
		}

		var msgs []provider.Message
		select {
		case msgs = <-msgsCh:
		case <-time.After(time.Second):
			t.Fatal("Messages() did not return after turn finished")
		}

		var sawUser, sawAssistant bool
		for _, m := range msgs {
			if m.Role == provider.RoleUser && m.Content == concurrencyUserMsg {
				sawUser = true
			}
			if m.Role == provider.RoleModel && m.Content == concurrencyAssistantMsg {
				sawAssistant = true
			}
		}
		if !sawUser {
			t.Errorf("snapshot missing user message; got %+v", msgs)
		}
		if !sawAssistant {
			t.Errorf("snapshot missing assistant message; got %+v", msgs)
		}
	})

	t.Run("close cancels in-flight turn and removes session", func(t *testing.T) {
		p := newGatedProvider()
		sm := newConcurrencyTestSM(t, p)
		sess := sm.Create()

		turnDone := startGatedTurn(sm, p, sess.ID, "hi")

		closeReturned := make(chan struct{})
		go func() {
			sm.Close(context.Background(), sess.ID)
			close(closeReturned)
		}()

		select {
		case err := <-turnDone:
			if err == nil {
				t.Error("expected turn to return error after Close")
			}
		case <-time.After(time.Second):
			t.Fatal("turn did not return after Close")
		}

		select {
		case <-closeReturned:
		case <-time.After(time.Second):
			t.Fatal("Close did not return")
		}

		if sm.Get(sess.ID) != nil {
			t.Error("expected session to be removed after Close")
		}
	})

	t.Run("interrupt unblocks in-flight turn", func(t *testing.T) {
		p := newGatedProvider()
		sm := newConcurrencyTestSM(t, p)
		sess := sm.Create()

		turnDone := startGatedTurn(sm, p, sess.ID, "hi")

		sm.Interrupt(sess.ID)

		select {
		case err := <-turnDone:
			if err == nil {
				t.Error("expected turn to return error after Interrupt")
			}
		case <-time.After(time.Second):
			t.Fatal("turn did not return after Interrupt")
		}

		readDone := make(chan struct{})
		go func() {
			_ = sess.Messages()
			_ = sess.Todos()
			close(readDone)
		}()
		select {
		case <-readDone:
		case <-time.After(time.Second):
			t.Fatal("post-interrupt readers did not unblock")
		}
	})

	t.Run("stress: hammered readers and cancellers do not race or panic", func(t *testing.T) {
		sm := newConcurrencyTestSM(t, slowProvider{delay: 50 * time.Millisecond})
		sess := sm.Create()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sm.RunTurnStream(context.Background(), sess.ID, "hi", nil, nil, nil, nil)
		}()

		const hammers = 16
		const iters = 10
		for range hammers {
			wg.Add(3)
			go func() {
				defer wg.Done()
				for range iters {
					_ = sess.Messages()
				}
			}()
			go func() {
				defer wg.Done()
				for range iters {
					_ = sess.Todos()
				}
			}()
			go func() {
				defer wg.Done()
				for range iters {
					sm.Interrupt(sess.ID)
				}
			}()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.Close(context.Background(), sess.ID)
		}()

		wg.Wait()
	})
}
