package Milky_go_sdk

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const version = "1.0.1"

const milkyVersion = "1.0.0"

func New(wsGateway string, restGateway string, token string, logger Logger) (s *Session, err error) {
	// Create an empty Session interface.
	s = &Session{
		ShouldReconnectOnError: true,
		MaxRestRetries:         3,
		Client:                 &http.Client{Timeout: 20 * time.Second},
		Dialer:                 websocket.DefaultDialer,
		UserAgent:              "MilkyGo (" + "v" + version + ") (" + "Milky " + milkyVersion + ")",
		LastHeartbeatAck:       time.Now().UTC(),
		WSGateway:              wsGateway,
		RestGateway:            restGateway,
		Logger:                 logger,
		Token:                  token,
		apiEndpoints:           newAPIEndpoints(restGateway),
	}

	return
}
