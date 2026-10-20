package Milky_go_sdk

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var ErrNoGateway = errors.New("no gateway URL set, cannot open websocket")

// ErrWSAlreadyOpen is thrown when you attempt to open
// a websocket that already is open.
var ErrWSAlreadyOpen = errors.New("web socket already opened")

// ErrWSNotFound is thrown when you attempt to use a websocket
// that doesn't exist
var ErrWSNotFound = errors.New("no websocket connection exists")

// ErrWSShardBounds is thrown when you try to use a shard ID that is
// more than the total shard count
var ErrWSShardBounds = errors.New("ShardID must be less than ShardCount")

type resumePacket struct {
	Op   int `json:"op"`
	Data struct {
		Token     string `json:"token"`
		SessionID string `json:"session_id"`
		Sequence  int64  `json:"seq"`
	} `json:"d"`
}

// Open creates a websocket connection
func (s *Session) Open() error {
	s.Logger.Debugf("called")

	var err error

	// Prevent Open or other major Session functions from
	// being called while Open is still running.
	s.Lock()
	defer s.Unlock()

	// If the websock is already open, bail out here.
	if s.wsConn != nil {
		return ErrWSAlreadyOpen
	}

	// Get the gateway to use for the Websocket connection
	if s.WSGateway == "" {
		return ErrNoGateway
	}

	// Connect to the WSGateway
	s.Logger.Debugf("connecting to gateway %s", s.WSGateway)
	header := http.Header{}
	addr := s.WSGateway
	if s.Token != "" {
		addr = s.WSGateway + "?access_token=" + s.Token
	}
	s.wsConn, _, err = s.Dialer.Dial(addr, header)
	if err != nil {
		s.Logger.Errorf("error connecting to gateway %s, %s", s.WSGateway, err)
		s.wsConn = nil // Just to be safe.
		return err
	}

	s.wsConn.SetCloseHandler(func(code int, text string) error {
		return nil
	})

	defer func() {
		if err != nil {
			_ = s.wsConn.Close()
			s.wsConn = nil
		}
	}()
	s.LastHeartbeatAck = time.Now().UTC()
	var h helloOp
	h.HeartbeatInterval = 41250 * time.Millisecond

	s.listening = make(chan interface{})

	// Start sending heartbeats and reading messages
	go s.heartbeat(s.wsConn, s.listening, h.HeartbeatInterval)
	go s.listen(s.wsConn, s.listening)

	s.Logger.Debug("exiting")
	return nil
}

// listen polls the websocket connection for events, it will stop when the
// listening channel is closed, or an error occurs.
func (s *Session) listen(wsConn *websocket.Conn, listening <-chan interface{}) {

	s.Logger.Debug("called")

	for {

		messageType, message, err := wsConn.ReadMessage()

		if err != nil {

			// Detect if we have been closed manually. If a Close() has already
			// happened, the websocket we are listening on will be different to
			// the current session.
			s.RLock()
			sameConnection := s.wsConn == wsConn
			s.RUnlock()

			if sameConnection {

				s.Logger.Warnf("error reading from gateway %s websocket, %s", s.WSGateway, err)
				// There has been an error reading, close the websocket so that
				// OnDisconnect event is emitted.
				err := s.Close()
				if err != nil {
					s.Logger.Warnf("error closing session connection, %s", err)
				}

				s.Logger.Infof("calling reconnect() now")
				s.reconnect()
			}

			return
		}

		select {

		case <-listening:
			return

		default:
			s.onEvent(messageType, message)

		}
	}
}

type heartbeatOp struct {
	Op   int   `json:"op"`
	Data int64 `json:"d"`
}

type helloOp struct {
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
}

// FailedHeartbeatAcks is the Number of heartbeat intervals to wait until forcing a connection restart.
const FailedHeartbeatAcks time.Duration = 5 * time.Millisecond

// HeartbeatLatency returns the latency between heartbeat acknowledgement and heartbeat send.
func (s *Session) HeartbeatLatency() time.Duration {

	return s.LastHeartbeatAck.Sub(s.LastHeartbeatSent)

}

// heartbeat sends regular heartbeats to Server so it knows the client
// is still connected.
func (s *Session) heartbeat(wsConn *websocket.Conn, listening <-chan interface{}, heartbeatIntervalMsec time.Duration) {

	s.Logger.Debug("called")

	if listening == nil || wsConn == nil {
		return
	}

	var err error
	ticker := time.NewTicker(heartbeatIntervalMsec * time.Millisecond)
	defer ticker.Stop()

	for {
		s.RLock()
		last := s.LastHeartbeatAck
		s.RUnlock()
		sequence := 0
		s.Logger.Debugf("sending gateway websocket heartbeat seq %d", sequence)
		s.wsMutex.Lock()
		s.LastHeartbeatSent = time.Now().UTC()
		err = wsConn.WriteJSON(heartbeatOp{1, int64(sequence)})
		s.wsMutex.Unlock()
		if err != nil || time.Now().UTC().Sub(last) > (heartbeatIntervalMsec*FailedHeartbeatAcks) {
			if err != nil {
				s.Logger.Debugf("error sending heartbeat to gateway %s, %s", s.WSGateway, err)
			} else {
				s.Logger.Errorf("haven't gotten a heartbeat ACK in %v, triggering a reconnection", time.Now().UTC().Sub(last))
			}
			s.Close()
			s.reconnect()
			return
		}
		s.Lock()
		s.Unlock()

		select {
		case <-ticker.C:
			// continue loop and send heartbeat
		case <-listening:
			return
		}
	}
}

// onEvent is the "event handler" for all messages received on the
// WSGateway API websocket connection.
//
// If you use the AddHandler() function to register a handler for a
// specific event this function will pass the event along to that handler.
//
// If you use the AddHandler() function to register a handler for the
// "OnEvent" event then all events will be passed to that handler.
func (s *Session) onEvent(messageType int, message []byte) (*Event, error) {

	var err error
	var reader io.Reader
	reader = bytes.NewBuffer(message)

	// If this is a compressed message, uncompress it.
	if messageType == websocket.BinaryMessage {

		z, err2 := zlib.NewReader(reader)
		if err2 != nil {
			s.Logger.Errorf("error uncompressing websocket message, %s", err)
			return nil, err2
		}

		defer func() {
			err3 := z.Close()
			if err3 != nil {
				s.Logger.Warnf("error closing zlib, %s", err)
			}
		}()

		reader = z
	}

	// read earlier data from the reader
	var rawData []byte
	if rawData, err = io.ReadAll(reader); err != nil {
		s.Logger.Errorf("error reading websocket message, %s", err)
		return nil, err
	}
	// print in debug mode
	s.Logger.Debugf("received websocket message: %s", string(rawData))

	// Create a new buffer to hold the raw data.
	var rawDataBuffer bytes.Buffer
	rawDataBuffer.Write(rawData)

	// Decode the event into an Event struct.
	var e *Event
	decoder := json.NewDecoder(&rawDataBuffer)
	if err = decoder.Decode(&e); err != nil {
		s.Logger.Errorf("error decoding websocket message, %s", err)
		return e, err
	}

	// Map event to registered event handlers and pass it along to any registered handlers.
	if eh, ok := registeredInterfaceProviders[e.Type]; ok {
		e.Struct = eh.New()

		// Attempt to unmarshal our event.
		if err = json.Unmarshal(e.RawData, e.Struct); err != nil {
			s.Logger.Errorf("error unmarshalling %s event, %s", e.Type, err)
		}

		s.handleEvent(e.Type, e.Struct)
	} else {
		s.Logger.Warnf("unknown event: Type: %s, Data: %s", e.Type, string(e.RawData))
	}
	// s.handleEvent(eventEventType, e)

	return e, nil
}

func (s *Session) reconnect() {

	s.Logger.Debugf("called")

	var err error

	if s.ShouldReconnectOnError {

		wait := time.Duration(1)

		for {
			s.Logger.Info("trying to reconnect to gateway")

			err = s.Open()
			if err == nil {
				s.Logger.Info("successfully reconnected to gateway")
				return
			}

			// Certain race conditions can call reconnect() twice. If this happens, we
			// just break out of the reconnect loop
			if errors.Is(err, ErrWSAlreadyOpen) {
				s.Logger.Info("Websocket already exists, no need to reconnect")
				return
			}

			s.Logger.Errorf("error reconnecting to gateway, %s", err)

			<-time.After(wait * time.Second)
			wait *= 2
			if wait > 600 {
				wait = 600
			}
		}
	}
}

// Close closes a websocket and stops all listening/heartbeat goroutines.
func (s *Session) Close() error {
	return s.CloseWithCode(websocket.CloseNormalClosure)
}

// CloseWithCode closes a websocket using the provided closeCode and stops all
// listening/heartbeat goroutines.
func (s *Session) CloseWithCode(closeCode int) (err error) {

	s.Logger.Debug("called")
	s.Lock()

	if s.listening != nil {
		s.Logger.Info("closing listening channel")
		close(s.listening)
		s.listening = nil
	}

	if s.wsConn != nil {

		s.Logger.Info("sending close frame")
		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		s.wsMutex.Lock()
		err = s.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, ""))
		s.wsMutex.Unlock()
		if err != nil {
			s.Logger.Warnf("error closing websocket, %s", err)
		}

		time.Sleep(1 * time.Second)

		s.Logger.Infof("closing gateway websocket")
		err = s.wsConn.Close()
		if err != nil {
			s.Logger.Warnf("error closing websocket, %s", err)
		}

		s.wsConn = nil
	}

	s.Unlock()

	return
}
