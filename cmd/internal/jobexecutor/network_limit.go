package jobexecutor

import (
	"net"
	"sync"
)

type limitedListener struct {
	net.Listener
	slots  chan struct{}
	closed chan struct{}
	once   sync.Once
}
type limitedConnection struct {
	net.Conn
	release func()
	once    sync.Once
}

func limitNetworkListener(listener net.Listener, maximum int) net.Listener {
	return &limitedListener{Listener: listener, slots: make(chan struct{}, maximum), closed: make(chan struct{})}
}
func (l *limitedListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.closed:
		return nil, net.ErrClosed
	}
	conn, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &limitedConnection{Conn: conn, release: func() { <-l.slots }}, nil
}
func (l *limitedListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return l.Listener.Close()
}
func (c *limitedConnection) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
