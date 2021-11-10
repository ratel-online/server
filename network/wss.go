package network

import (
    "github.com/gorilla/websocket"
    "github.com/ratel-online/core/protocol"
    "log"
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
    log.Printf("Websocket server listener on %s\n", w.addr)
    return http.ListenAndServe(w.addr, nil)
}

func serveWs(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println(err)
        return
    }
    handle(protocol.NewWebsocketReadWriteCloser(conn))
}
