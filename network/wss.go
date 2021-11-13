package network

import (
    "github.com/gorilla/websocket"
    "github.com/ratel-online/core/log"
    "github.com/ratel-online/core/protocol"
    "net/http"
)

type Websocket struct {
    addr string
}

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func NewWebsocketServer(addr string) Websocket {
    return Websocket{addr: addr}
}

func (w Websocket) Serve() error {
    http.HandleFunc("/ws", serveWs)
    log.Infof("Websocket server listener on %s\n", w.addr)
    return http.ListenAndServe(w.addr, nil)
}

func serveWs(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Error(err)
        return
    }
    err = handle(protocol.NewWebsocketReadWriteCloser(conn))
    if err != nil{
        log.Error(err)
    }
}
