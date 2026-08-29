package kernel

import (
	"fmt"
	"sync/atomic"

	"github.com/beanruntime/bean/internal/appir"
)

type Kernel struct{ active atomic.Pointer[appir.App] }

func New() *Kernel { return &Kernel{} }
func (k *Kernel) Activate(a *appir.App) error {
	if a == nil {
		return fmt.Errorf("cannot activate nil AppIR")
	}
	if e := a.ValidateFormat(); e != nil {
		return e
	}
	clone, e := a.Clone()
	if e != nil {
		return e
	}
	k.active.Store(clone)
	return nil
}
func (k *Kernel) Active() (*appir.App, bool) { a := k.active.Load(); return a, a != nil }
