package udp

import (
	"net"
	"sync"
	"time"

	"github.com/go-gost/core/common/bufpool"
	"github.com/go-gost/core/logger"
)

type listener struct {
	addr           net.Addr
	conn           net.PacketConn
	cqueue         chan net.Conn
	readQueueSize  int
	readBufferSize int
	connPool       *ConnPool
	mux            sync.Mutex
	closed         chan struct{}
	errChan        chan error
	logger         logger.Logger
}

func NewListener(conn net.PacketConn, addr net.Addr, backlog, dataQueueSize, dataBufferSize int, ttl time.Duration, logger logger.Logger) net.Listener {
	ln := &listener{
		conn:           conn,
		addr:           addr,
		cqueue:         make(chan net.Conn, backlog),
		connPool:       NewConnPool(ttl).WithLogger(logger),
		readQueueSize:  dataQueueSize,
		readBufferSize: dataBufferSize,
		closed:         make(chan struct{}),
		errChan:        make(chan error, 1),
		logger:         logger,
	}
	go ln.listenLoop()

	return ln
}

func (ln *listener) Accept() (conn net.Conn, err error) {
	select {
	case conn = <-ln.cqueue:
		return
	case <-ln.closed:
		return nil, net.ErrClosed
	case err = <-ln.errChan:
		if err == nil {
			err = net.ErrClosed
		}
		return
	}
}

func (ln *listener) listenLoop() {
	for {
		select {
		case <-ln.closed:
			return
		default:
		}

		b := bufpool.Get(ln.readBufferSize)

		n, raddr, err := ln.conn.ReadFrom(*b)
		if err != nil {
			ln.errChan <- err
			close(ln.errChan)
			return
		}

		c := ln.getConn(raddr)
		if c == nil {
			bufpool.Put(b)
			continue
		}

		if err := c.WriteQueue((*b)[:n]); err != nil {
			ln.logger.Warn("data discarded: ", err)
		}
	}
}

func (ln *listener) Addr() net.Addr {
	return ln.addr
}

func (ln *listener) Close() error {
	select {
	case <-ln.closed:
	default:
		close(ln.closed)
		ln.conn.Close()
		ln.connPool.Close()
	}

	return nil
}

func (ln *listener) getConn(raddr net.Addr) *Conn {
	ln.mux.Lock()
	defer ln.mux.Unlock()

	c, ok := ln.connPool.Get(raddr.String())
	if ok {
		return c
	}

	c = NewConn(ln.conn, ln.addr, raddr, ln.readQueueSize)
	select {
	case ln.cqueue <- c:
		ln.connPool.Set(raddr.String(), c)
		return c
	default:
		c.Close()
		ln.logger.Warnf("connection queue is full, client %s discarded", raddr)
		return nil
	}
}
