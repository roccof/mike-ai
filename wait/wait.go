package wait

import (
	"context"
	"sync"
)

type Group struct {
	wg sync.WaitGroup
}

func (g *Group) Wait() {
	g.wg.Wait()
}

func (g *Group) Start(ctx context.Context, f func(context.Context)) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		f(ctx)
	}()
}
