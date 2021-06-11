package Objectpool

import(
	"sync"
	"sync/atomic"
)

type ReferenceCountedPool struct {
	pool       *sync.Pool
	factory    func() ReferenceCountable
	returned   uint32
	allocated  uint32
	referenced uint32
}

// Method to create a new pool
func NewReferenceCountedPool(factory func(referenceCounter ReferenceCounter) ReferenceCountable, reset func(interface{}) error) *ReferenceCountedPool {
	p := new(ReferenceCountedPool)
	p.pool = new(sync.Pool)
	p.pool.New = func() interface{} {
		// Incrementing allocated count
		atomic.AddUint32(&p.allocated, 1)
		c := factory(ReferenceCounter{
			count:       new(uint32),
			destination: p.pool,
			released:    &p.returned,
			reset:       reset,
			id:          p.allocated,
		})
		return c
	}
	return p
}

// Method to get new object
func (p *ReferenceCountedPool) Get() ReferenceCountable {
	c := p.pool.Get().(ReferenceCountable)
	c.SetInstance(c)
	atomic.AddUint32(&p.referenced, 1)
	c.IncrementReferenceCount()
	return c
}

// Method to return reference counted pool stats
func (p *ReferenceCountedPool) Stats() map[string]interface{} {
	return map[string]interface{}{"allocated": p.allocated, "referenced": p.referenced, "returned": p.returned}
}