package Milky_go_sdk

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Session struct {
	sync.RWMutex

	Token string // rest authentication token
	// stores sessions current websocket gateway
	WSGateway string
	// stores sessions current rest gateway
	RestGateway string

	apiEndpoints *apiEndpoints

	LogLevel int

	// Should the session reconnect the websocket on errors.
	ShouldReconnectOnError bool

	Logger Logger

	// Whether or not to call event handlers synchronously.
	// e.g. false = launch event handlers in their own goroutines.
	SyncEvents bool

	// Exposed but should not be modified by User.

	// Max number of REST API retries
	MaxRestRetries int

	// The http client used for REST requests
	Client *http.Client

	// The dialer used for WebSocket connection
	Dialer *websocket.Dialer

	// The user agent used for REST APIs
	UserAgent string

	// Stores the last HeartbeatAck that was received (in UTC)
	LastHeartbeatAck time.Time

	// Stores the last Heartbeat sent (in UTC)
	LastHeartbeatSent time.Time

	// Event handlers
	handlersMu   sync.RWMutex
	handlers     map[string][]*eventHandlerInstance
	onceHandlers map[string][]*eventHandlerInstance

	// The websocket connection.
	wsConn *websocket.Conn

	// When nil, the session is not listening.
	listening chan interface{}

	// stores session ID of current WSGateway connection
	sessionID string

	// used to make sure gateway websocket writes do not happen concurrently
	wsMutex sync.Mutex
}

type APIErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
