package Objectpool

import(
	"sync"
	"sync/atomic"
	"reflect"
)

type ReferenceCountable interface {
	// Method to set the current instance
	SetInstance(i interface{})
	// Method to increment the reference count
	IncrementReferenceCount()
	// Method to decrement reference count
	DecrementReferenceCount()
}

type ReferenceCounter struct {
	count       *uint32                 `sql:"-" json:"-" yaml:"-"`
	destination *sync.Pool              `sql:"-" json:"-" yaml:"-"`
	released    *uint32                 `sql:"-" json:"-" yaml:"-"`
	Instance    interface{}             `sql:"-" json:"-" yaml:"-"`
	reset       func(interface{}) error `sql:"-" json:"-" yaml:"-"`
	id          uint32                  `sql:"-" json:"-" yaml:"-"`
}

// Method to increment a reference
func (r *ReferenceCounter) IncrementReferenceCount() {
	atomic.AddUint32(r.count, 1)
}

// Method to decrement a reference
// If the reference count goes to zero, the object is put back inside the pool
func (r *ReferenceCounter) DecrementReferenceCount() {
	if atomic.LoadUint32(r.count) == 0 {
		panic("this should not happen =>" + reflect.TypeOf(r.Instance).String())
	}
	if atomic.AddUint32(r.count, ^uint32(0)) == 0 {
		atomic.AddUint32(r.released, 1)
		if err := r.reset(r.Instance); err != nil {
			panic("error while resetting an instance => " + err.Error())
		}
		r.destination.Put(r.Instance)
		r.Instance = nil
	}
}

// Method to set the current instance
func (r *ReferenceCounter) SetInstance(i interface{}) {
	r.Instance = i
}