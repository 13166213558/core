package chain

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/go-gost/core/common/net/dialer"
	"github.com/go-gost/core/common/util/udp"
	"github.com/go-gost/core/connector"
	"github.com/go-gost/core/logger"
)

var (
	ErrEmptyRoute = errors.New("empty route")
)

type Route struct {
	nodes    []*Node
	ifceName string
	logger   logger.Logger
}

func (r *Route) addNode(node *Node) {
	r.nodes = append(r.nodes, node)
}

func (r *Route) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	if r.Len() == 0 {
		netd := dialer.NetDialer{
			Timeout: 30 * time.Second,
		}
		if r != nil {
			netd.Interface = r.ifceName
		}
		return netd.Dial(ctx, network, address)
	}

	conn, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}

	cc, err := r.GetNode(r.Len()-1).Transport.Connect(ctx, conn, network, address)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return cc, nil
}

func (r *Route) Bind(ctx context.Context, network, address string, opts ...connector.BindOption) (net.Listener, error) {
	if r.Len() == 0 {
		return r.bindLocal(ctx, network, address, opts...)
	}

	conn, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}

	ln, err := r.GetNode(r.Len()-1).Transport.Bind(ctx, conn, network, address, opts...)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return ln, nil
}

func (r *Route) connect(ctx context.Context) (conn net.Conn, err error) {
	if r.Len() == 0 {
		return nil, ErrEmptyRoute
	}

	network := "ip"
	node := r.nodes[0]

	addr, err := resolve(ctx, network, node.Addr, node.Resolver, node.Hosts, r.logger)
	if err != nil {
		node.Marker.Mark()
		return
	}
	cc, err := node.Transport.Dial(ctx, addr)
	if err != nil {
		node.Marker.Mark()
		return
	}

	cn, err := node.Transport.Handshake(ctx, cc)
	if err != nil {
		cc.Close()
		node.Marker.Mark()
		return
	}
	node.Marker.Reset()

	preNode := node
	for _, node := range r.nodes[1:] {
		addr, err = resolve(ctx, network, node.Addr, node.Resolver, node.Hosts, r.logger)
		if err != nil {
			cn.Close()
			node.Marker.Mark()
			return
		}
		cc, err = preNode.Transport.Connect(ctx, cn, "tcp", addr)
		if err != nil {
			cn.Close()
			node.Marker.Mark()
			return
		}
		cc, err = node.Transport.Handshake(ctx, cc)
		if err != nil {
			cn.Close()
			node.Marker.Mark()
			return
		}
		node.Marker.Reset()

		cn = cc
		preNode = node
	}

	conn = cn
	return
}

func (r *Route) Len() int {
	if r == nil {
		return 0
	}
	return len(r.nodes)
}

func (r *Route) GetNode(index int) *Node {
	if r.Len() == 0 || index < 0 || index >= len(r.nodes) {
		return nil
	}
	return r.nodes[index]
}

func (r *Route) Path() (path []*Node) {
	if r == nil || len(r.nodes) == 0 {
		return nil
	}

	for _, node := range r.nodes {
		if node.Transport != nil && node.Transport.route != nil {
			path = append(path, node.Transport.route.Path()...)
		}
		path = append(path, node)
	}
	return
}

func (r *Route) bindLocal(ctx context.Context, network, address string, opts ...connector.BindOption) (net.Listener, error) {
	options := connector.BindOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	switch network {
	case "tcp", "tcp4", "tcp6":
		addr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return nil, err
		}
		return net.ListenTCP(network, addr)
	case "udp", "udp4", "udp6":
		addr, err := net.ResolveUDPAddr(network, address)
		if err != nil {
			return nil, err
		}
		conn, err := net.ListenUDP(network, addr)
		if err != nil {
			return nil, err
		}
		logger := logger.Default().WithFields(map[string]any{
			"network": network,
			"address": address,
		})
		ln := udp.NewListener(conn, addr,
			options.Backlog, options.UDPDataQueueSize, options.UDPDataBufferSize,
			options.UDPConnTTL, logger)
		return ln, err
	default:
		err := fmt.Errorf("network %s unsupported", network)
		return nil, err
	}
}
