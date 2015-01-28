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

func (_ waitStack) push(_ *waiter) {}
func (_ waitStack) pop(c *Client) *waiter {
	w := &waiter{parent: c}
	w.done.Lock()
	return w
}

func (_ connStack) pop() *connWrapper   { return &connWrapper{} }
func (_ connStack) push(_ *connWrapper) {}