package network

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/slclub/leaf/log"
	"net"
	"sync"
)

type WebsocketConnSet map[*websocket.Conn]struct{}

type WSConn struct {
	sync.Mutex
	conn      *websocket.Conn
	writeChan chan []byte
	maxMsgLen uint32
	closeFlag bool
	StopChan  chan struct{}
}

func newWSConn(conn *websocket.Conn, pendingWriteNum int, maxMsgLen uint32) *WSConn {
	wsConn := new(WSConn)
	wsConn.conn = conn
	wsConn.writeChan = make(chan []byte, pendingWriteNum)
	wsConn.maxMsgLen = maxMsgLen
	wsConn.StopChan = make(chan struct{})

	go func() {
		defer wsConn.Close()
		for {
			select {
			case <-wsConn.StopChan:

				return
			case b := <-wsConn.writeChan:
				if b == nil {
					return
				}

				err := conn.WriteMessage(websocket.BinaryMessage, b)
				if err != nil {
					return
				}
			}

		}

		//conn.Close()
		//wsConn.Lock()

		//wsConn.Unlock()
	}()

	return wsConn
}

func (wsConn *WSConn) doDestroy() {
	//wsConn.conn.UnderlyingConn().(*net.TCPConn).SetLinger(0)
	//wsConn.conn.Close()
	//
	//if !wsConn.closeFlag {
	//	close(wsConn.writeChan)
	//	wsConn.closeFlag = true
	//}
}

func (wsConn *WSConn) Destroy() {
	wsConn.Lock()
	defer wsConn.Unlock()

	wsConn.doDestroy()
}

func (wsConn *WSConn) Close() {
	wsConn.Lock()
	defer wsConn.Unlock()
	if wsConn.closeFlag {
		return
	}
	//wsConn.doWrite(nil)
	close(wsConn.StopChan)
	close(wsConn.writeChan)
	wsConn.conn.Close()
	wsConn.closeFlag = true
	//wsConn.conn.UnderlyingConn().(*net.TCPConn).SetLinger(0)
}

func (wsConn *WSConn) doWrite(b []byte) {
	if len(wsConn.writeChan) == cap(wsConn.writeChan) {
		log.Debug("close conn: channel full")
		wsConn.doDestroy()
		return
	}

	wsConn.writeChan <- b
}

func (wsConn *WSConn) LocalAddr() net.Addr {
	return wsConn.conn.LocalAddr()
}

func (wsConn *WSConn) RemoteAddr() net.Addr {
	return wsConn.conn.RemoteAddr()
}

// goroutine not safe
func (wsConn *WSConn) ReadMsg() ([]byte, error) {
	if wsConn.closeFlag {
		return nil, errors.New("closed wsconn")
	}
	_, b, err := wsConn.conn.ReadMessage()
	return b, err
}

// args must not be modified by the others goroutines
func (wsConn *WSConn) WriteMsg(args ...[]byte) error {
	wsConn.Lock()
	defer wsConn.Unlock()
	if wsConn.closeFlag {
		return nil
	}

	// get len
	var msgLen uint32
	for i := 0; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}

	// check len
	if msgLen > wsConn.maxMsgLen {
		return errors.New("message too long")
	} else if msgLen < 1 {
		return errors.New("message too short")
	}

	// don't copy
	if len(args) == 1 {
		wsConn.doWrite(args[0])
		return nil
	}

	// merge the args
	msg := make([]byte, msgLen)
	l := 0
	for i := 0; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += len(args[i])
	}

	wsConn.doWrite(msg)

	return nil
}
