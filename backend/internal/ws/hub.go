package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

// NewHub 功能：创建一个 WS Hub，用于维护连接与广播消息。
// 参数/返回：无入参；返回 *Hub。
// 失败场景：无。
// 副作用：无。
func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// Broadcast 功能：向所有已连接的客户端广播一条消息（TextMessage）。
// 参数/返回：msg 为已编码的 JSON 字节串；无返回值。
// 失败场景：单个客户端发送队列满时会丢弃该客户端的该条消息（用于避免阻塞）。
// 副作用：向网络连接写入数据。
func (h *Hub) Broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// Slow client. Drop messages and rely on log tail for catch-up.
		}
	}
}

// Serve 功能：在当前连接上运行 WS 会话（阻塞直到连接关闭）。
// 参数/返回：conn 为升级后的 websocket.Conn；无返回值。
// 失败场景：连接异常关闭时退出；不会向上返回错误。
// 副作用：注册连接到 Hub、启动写协程、持续读以维持连接。
func (h *Hub) Serve(conn *websocket.Conn) {
	c := &client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump()
	c.readPump()

	h.mu.Lock()
	delete(h.clients, c)
	close(c.send)
	h.mu.Unlock()
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

const (
	writeWait = 10 * time.Second
	pongWait  = 60 * time.Second
	pingEvery = (pongWait * 9) / 10
)

func (c *client) readPump() {
	defer func() {
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingEvery)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
