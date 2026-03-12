package repolib

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

type sseSubscriber struct {
	ch chan []byte
}

// SSEBroker 功能：为 Repo Library 提供进程内 SSE 发布订阅，用于向 UI 推送状态变更事件。
// 参数/返回：通过 Subscribe 获取只读 channel；通过 Broadcast 广播事件帧。
// 失败场景：慢客户端队列满时丢弃该条消息，避免阻塞广播。
// 副作用：无网络副作用，仅在内存中分发字节帧。
type SSEBroker struct {
	mu     sync.RWMutex
	subs   map[*sseSubscriber]struct{}
	nextID uint64
}

func NewSSEBroker() *SSEBroker {
	return &SSEBroker{subs: make(map[*sseSubscriber]struct{})}
}

// Subscribe 功能：注册一个订阅并返回订阅句柄与消息 channel。
// 参数/返回：无入参；返回 subscriber 与只读 channel。
// 失败场景：无。
// 副作用：向 subs 注册一个订阅。
func (b *SSEBroker) Subscribe() (*sseSubscriber, <-chan []byte) {
	if b == nil {
		ch := make(chan []byte)
		close(ch)
		return &sseSubscriber{ch: ch}, ch
	}
	sub := &sseSubscriber{ch: make(chan []byte, 64)}
	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return sub, sub.ch
}

// Unsubscribe 功能：取消订阅并关闭 channel。
// 参数/返回：sub 为 Subscribe 返回的订阅句柄；无返回值。
// 失败场景：重复取消不会 panic。
// 副作用：从 subs 移除并关闭 channel。
func (b *SSEBroker) Unsubscribe(sub *sseSubscriber) {
	if b == nil || sub == nil {
		return
	}
	b.mu.Lock()
	if _, ok := b.subs[sub]; ok {
		delete(b.subs, sub)
		close(sub.ch)
	}
	b.mu.Unlock()
}

// Broadcast 功能：向所有订阅者广播一个 SSE 事件帧。
// 参数/返回：event 为 SSE event name；payload 为 JSON 数据；无返回值。
// 失败场景：payload 无法序列化时丢弃。
// 副作用：向订阅者 channel 写入数据帧。
func (b *SSEBroker) Broadcast(event string, payload any) {
	if b == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	id := atomic.AddUint64(&b.nextID, 1)
	frame := []byte(fmt.Sprintf("id: %d\nevent: %s\ndata: %s\n\n", id, event, string(data)))

	b.mu.RLock()
	defer b.mu.RUnlock()
	for sub := range b.subs {
		select {
		case sub.ch <- frame:
		default:
			// slow subscriber: drop to avoid blocking
		}
	}
}

