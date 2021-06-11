package worker

import (
	"net"
	"sync"
	"errors"
	"fmt"
	"time"
)

var (
	errPoolIsClose = errors.New("Connection pool has been closed")
	errTimeOut      = errors.New("Get Connection timeout")
	errContextClose = errors.New("Get Connection close by context")
	errMaxNumberReached = errors.New("Connection pool reached max number of sockets")

	pool *ConnectionPool
)

type Conn struct{
	Sock 	net.Conn
	Pool	*ConnectionPool
}

func (c *Conn)Destroy() error{
	if c.Pool == nil{
		return errors.New("Connection doesnt belong to any pool")
	}
	err := c.Pool.Remove(c.Sock)
	if err != nil{
		return err
	}
	c.Pool = nil
	return nil
}

func (c *Conn)Close() error{
	if c.Pool == nil{
		return errors.New("Connection doesnt belong to any pool")
	}
	return c.Pool.Put(c.Sock)

}


type ConnectionPool	struct{
	ConnPool 		chan net.Conn
	Lock			sync.Mutex
	MaxNumber		int
	CurrentNumber	int
	MinNumber 		int
	closed		bool
	connCreator  	func() (net.Conn, error)
}

func (p *ConnectionPool)Init() error{
	fmt.Println("sockets are being created")
	for i := 0; i < p.MinNumber; i++ {
		fmt.Println("socket: %d", i)
		if conn, err := p.CreateConn(); err == nil{
			p.ConnPool <- conn
		}else{
			return err
		}
	}
	return nil
}

func NewConnectionPool(max int, connCreator func()( net.Conn, error), min int) error{
	fmt.Println("connection pool is being initialized")
	pool = &ConnectionPool{}
	pool.MaxNumber = max
	pool.CurrentNumber = 0
	pool.MinNumber = min
	pool.ConnPool = make(chan net.Conn, min)
	pool.closed = false
	pool.connCreator = connCreator
	
	err := pool.Init()
	if err != nil{
		return err
	}

	return nil
}

func (p *ConnectionPool)IsClosed() bool{
	p.Lock.Lock()
	val := p.closed
	p.Lock.Unlock()
	return val
}

func (p *ConnectionPool)Remove(conn net.Conn) error{
	if p.IsClosed() == true{
		return errPoolIsClose
	}
	
	p.Lock.Lock()
	p.CurrentNumber -= 1
	p.Lock.Unlock()

	return conn.Close()
	
}

func (p *ConnectionPool) Get(timeout time.Duration) (*Conn, error){
	if p.IsClosed() == true{
		return nil, errPoolIsClose
	}
	
	go func(){
		conn, err := p.CreateConn()
		if err != nil{
			return 
		}
		p.ConnPool <- conn
	}()

	select{
	case conn := <- p.ConnPool:
		return p.PackCon(conn), nil
	case <-time.After(timeout):
		return nil, errTimeOut
	}

}

func (p *ConnectionPool)Close() error{
	if p.IsClosed() == true{
		return errPoolIsClose
	}

	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.closed = true
	close(p.ConnPool)
	for conn := range p.ConnPool{
		conn.Close()
	}
	return nil

}

func(p *ConnectionPool)Put(conn net.Conn) error{
	if p.IsClosed() == true{
		return errPoolIsClose
	}

	if conn == nil{
		p.Lock.Lock()
		p.CurrentNumber -= 1
		p.Lock.Unlock()
		return fmt.Errorf("Cannot put nil socket back into pool")
	}

	select{
	case p.ConnPool <- conn:
		return nil
	default:
		return conn.Close()
	}
}

func (p *ConnectionPool)CreateConn()(net.Conn, error){
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if p.CurrentNumber >= p.MaxNumber{
		return nil, errMaxNumberReached
	}
	conn, errC := p.connCreator()
	if errC != nil{
		return nil, fmt.Errorf("Error creating a socket: %v", errC)
	}

	p.CurrentNumber += 1
	return conn, nil

}


func (p *ConnectionPool) PackCon(conn net.Conn) *Conn{
	return &Conn{Pool: p, Sock: conn}
}
