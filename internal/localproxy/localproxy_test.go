package localproxy

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestProxyForwardsBytes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()
	go echoServer(server)

	proxy, err := Start(ctx, Options{
		Dial: func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", server.Addr().String())
		},
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer proxy.Close()

	addr := strings.TrimPrefix(proxy.RuntimeBaseURL(), "stratum+tcp://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if line != "hello\n" {
		t.Fatalf("line = %q", line)
	}
}

func TestProxyCloseStopsListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxy, err := Start(ctx, Options{
		Dial: func(context.Context) (net.Conn, error) {
			return nil, context.Canceled
		},
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	addr := strings.TrimPrefix(proxy.RuntimeBaseURL(), "stratum+tcp://")
	if err := proxy.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	_, err = net.DialTimeout("tcp", addr, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected closed proxy listener to reject connections")
	}
}

func echoServer(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			_, _ = io.Copy(conn, conn)
		}()
	}
}
