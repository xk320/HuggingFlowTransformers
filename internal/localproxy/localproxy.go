package localproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type DialFunc func(context.Context) (net.Conn, error)

type Options struct {
	Dial        DialFunc
	IdleTimeout time.Duration
}

type Proxy struct {
	listener net.Listener
	options  Options
	done     chan struct{}
	once     sync.Once
}

func Start(ctx context.Context, options Options) (*Proxy, error) {
	if options.Dial == nil {
		return nil, fmt.Errorf("HFT local runtime proxy requires secure transport dialer")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start HFT local runtime proxy: %w", err)
	}
	proxy := &Proxy{listener: listener, options: options, done: make(chan struct{})}
	go proxy.accept(ctx)
	go func() {
		<-ctx.Done()
		_ = proxy.Close()
	}()
	return proxy, nil
}

func (p *Proxy) RuntimeBaseURL() string {
	return "stratum+tcp://" + p.listener.Addr().String()
}

func (p *Proxy) Close() error {
	var err error
	p.once.Do(func() {
		err = p.listener.Close()
		<-p.done
	})
	return err
}

func (p *Proxy) accept(ctx context.Context) {
	defer close(p.done)
	for {
		local, err := p.listener.Accept()
		if err != nil {
			return
		}
		go p.handle(ctx, local)
	}
}

func (p *Proxy) handle(ctx context.Context, local net.Conn) {
	defer local.Close()
	remote, err := p.options.Dial(ctx)
	if err != nil {
		return
	}
	defer remote.Close()
	if p.options.IdleTimeout > 0 {
		local = deadlineConn{Conn: local, idle: p.options.IdleTimeout}
		remote = deadlineConn{Conn: remote, idle: p.options.IdleTimeout}
	}
	copyBoth(local, remote)
}

func copyBoth(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(b, a)
		done <- struct{}{}
	}()
	<-done
	_ = a.Close()
	_ = b.Close()
	<-done
}

type deadlineConn struct {
	net.Conn
	idle time.Duration
}

func (c deadlineConn) Read(p []byte) (int, error) {
	_ = c.Conn.SetReadDeadline(time.Now().Add(c.idle))
	return c.Conn.Read(p)
}

func (c deadlineConn) Write(p []byte) (int, error) {
	_ = c.Conn.SetWriteDeadline(time.Now().Add(c.idle))
	return c.Conn.Write(p)
}
