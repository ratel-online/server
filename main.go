package main

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/server/bot"
	"github.com/ratel-online/server/network"
)

var (
	Wsport   int
	Tcpport  int
	BotAddr  string
	BotToken string
	BotGroup int64
)

func main() {
	flag.IntVar(&Wsport, "w", 9998, "WebsocketServer Port")
	flag.IntVar(&Tcpport, "t", 9999, "TcpServer Port")
	flag.StringVar(&BotAddr, "bot", "", "Bot connection address")
	flag.StringVar(&BotToken, "bot-token", "", "Bot token")
	flag.Int64Var(&BotGroup, "bot-group", 0, "Bot group ID")

	flag.Parse()
	// 连接机器人
	if BotAddr != "" && BotToken != "" && BotGroup != 0 {
		err := bot.Connect(BotAddr, BotToken, BotGroup)
		if err != nil {
			log.Panic(fmt.Sprintf("连接Bot失败: %v", err))
		}
		// 发送测试消息到 BotGroup 群
		err = bot.SendGroupMessage(BotGroup, "Server started!")
		if err != nil {
			log.Errorf("发送群消息失败: %v", err)
		} else {
			log.Infof("已发送群消息到 %d", BotGroup)
		}
		defer bot.Close()
	}

	async.Async(func() {
		wsServer := network.NewWebsocketServer(":" + strconv.Itoa(Wsport))
		log.Panic(wsServer.Serve())
	})

	server := network.NewTcpServer(":" + strconv.Itoa(Tcpport))
	log.Panic(server.Serve())
}