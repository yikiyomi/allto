package middleware

import "sync"

type SSEHub struct {
	mu    sync.RWMutex
	conns map[uint]chan []byte
}

// new创建sse hub
func NewSSEHub() *SSEHub {
	return &SSEHub{
		conns: make(map[uint]chan []byte),
	}
}

// 注册连接，返回该用户消息通道
func (h *SSEHub) Register(userID uint) chan []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan []byte, 10) //缓冲10条
	h.conns[userID] = ch
	return ch
}

// 断联
func (h *SSEHub) Unregister(userID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.conns[userID]; ok {
		close(ch)
		delete(h.conns, userID)
	}
}

// 推送信息
func (h *SSEHub) Push(userID uint, msg []byte) {
	h.mu.RLock()
	ch, ok := h.conns[userID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- msg:
	default: //通道满了扔，不堵塞
	}
}
