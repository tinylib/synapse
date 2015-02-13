// +build race

package synapse

// this file exports
// the same interface
// as stack.go, but
// no-ops instead.

var (
	waiters  = waitStack{}
	wrappers = connStack{}
)

type waitStack struct{}
type connStack struct{}

func (ws waitStack) push(_ *waiter) {}
func (ws waitStack) pop(c *Client) *waiter {
	w := &waiter{parent: c}
	w.done.Lock()
	return w
}

func (cs connStack) pop() *connWrapper   { return &connWrapper{} }
func (cs connStack) push(_ *connWrapper) {}
