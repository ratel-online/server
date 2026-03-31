package bot

import (
	"fmt"
	"github.com/Szzrain/Milky-go-sdk"
	"github.com/ratel-online/core/log"
)

// Session 全局机器人会话
var Session *Milky_go_sdk.Session

// GroupID 群ID
var GroupID int64

// Logger 实现 Milky_go_sdk 的 Logger 接口
type Logger struct{}

func (l *Logger) Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	_ = format
	_ = args
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	log.Infof(format, args...)
}

func (l *Logger) Info(args ...interface{}) {
	log.Info(fmt.Sprint(args...))
}

func (l *Logger) Error(args ...interface{}) {
	log.Error(fmt.Sprint(args...))
}

func (l *Logger) Debug(args ...interface{}) {
	_ = args
}

func (l *Logger) Warn(args ...interface{}) {
	log.Info(fmt.Sprint(args...))
}

// SendGroupMessage 发送群消息
func SendGroupMessage(groupID int64, content string) error {
	if Session == nil {
		return fmt.Errorf("bot not connected")
	}
	var message []Milky_go_sdk.IMessageElement
	message = append(message, &Milky_go_sdk.TextElement{Text: content})
	_, err := Session.SendGroupMessage(groupID, &message)
	return err
}

// Connect 连接机器人
func Connect(addr, token string, groupID int64) error {
	GroupID = groupID
	m, err := Milky_go_sdk.New("ws://"+addr+"/event", "http://"+addr+"/api", token, &Logger{})
	if err != nil {
		return fmt.Errorf("创建Bot会话失败: %v", err)
	}
	err = m.Open()
	if err != nil {
		return fmt.Errorf("连接Bot失败: %v", err)
	}
	Session = m
	log.Infof("Bot已连接: %s", addr)
	return nil
}

// Close 关闭机器人连接
func Close() {
	if Session != nil {
		Session.Close()
	}
}