package network

import (
    "github.com/ratel-online/core/log"
    "github.com/ratel-online/core/protocol"
    "github.com/ratel-online/core/util/async"
    "net"
)

type Tcp struct {
    addr string
}

func NewTcpServer(addr string) Tcp {
    return Tcp{addr: addr}
}

func (t Tcp) Serve() error {
    listener, err := net.Listen("tcp", t.addr)
    if err != nil {
        log.Error(err)
        return err
    }
    log.Infof("Tcp server listening on %s\n", t.addr)
    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Infof("listener.Accept err %v\n", err)
            continue
        }
        async.Async(func() {
            err := handle(protocol.NewTcpReadWriteCloser(conn))
            if err != nil{
                log.Error(err)
            }
        })
    }
}
