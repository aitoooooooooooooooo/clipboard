package clipboard

import (
	"sync"
	"testing"
	"time"
)

func TestWatcher_StartStop(t *testing.T) {
	w := NewWatcher(100 * time.Millisecond)

	if w.IsRunning() {
		t.Error("新创建的监听器不应处于运行状态")
	}

	w.Start()

	if !w.IsRunning() {
		t.Error("Start 后监听器应处于运行状态")
	}

	// 重复 Start 不应 panic
	w.Start()

	w.Stop()

	if w.IsRunning() {
		t.Error("Stop 后监听器不应处于运行状态")
	}

	// 重复 Stop 不应 panic
	w.Stop()
}

func TestWatcher_SetOnChange(t *testing.T) {
	w := NewWatcher(100 * time.Millisecond)

	called := false
	w.SetOnChange(func(event *ClipboardEvent) {
		called = true
	})

	if called {
		t.Error("SetOnChange 不应立即触发回调")
	}
}

func TestWatcher_ChangeDetection(t *testing.T) {
	// 此测试验证监听器的基本机制
	// 实际剪贴板操作依赖平台，此处测试内部状态管理
	w := NewWatcher(50 * time.Millisecond)

	var events []*ClipboardEvent
	var mu sync.Mutex

	w.SetOnChange(func(event *ClipboardEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	// 模拟第一次检查：设置初始哈希
	w.mu.Lock()
	w.lastHash = "initial-hash"
	w.mu.Unlock()

	// 调用 check（如果剪贴板内容与 lastHash 不同会触发回调）
	w.check()

	// 等待一小段时间确保回调执行
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	eventCount := len(events)
	mu.Unlock()

	// 如果当前剪贴板内容的哈希与 initial-hash 不同，应该收到一个事件
	// 如果相同，则没有事件（这也是正确的）
	if eventCount > 1 {
		t.Errorf("预期最多 1 个事件，实际收到 %d", eventCount)
	}

	if eventCount > 0 {
		mu.Lock()
		event := events[0]
		mu.Unlock()

		if event.ContentHash == "" {
			t.Error("事件的 ContentHash 不应为空")
		}
		if event.Payload == nil {
			t.Error("事件的 Payload 不应为 nil")
		}
	}
}

func TestWatcher_DuplicateDetection(t *testing.T) {
	w := NewWatcher(50 * time.Millisecond)

	callCount := 0
	var mu sync.Mutex

	w.SetOnChange(func(event *ClipboardEvent) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	// 设置一个与当前剪贴板可能不同的哈希来触发第一次回调
	w.mu.Lock()
	w.lastHash = "dummy-hash-for-test"
	w.mu.Unlock()

	// 第一次 check
	w.check()
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	firstCount := callCount
	mu.Unlock()

	// 第二次 check（内容未变化，不应触发回调）
	w.check()
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	secondCount := callCount
	mu.Unlock()

	if firstCount > 0 && secondCount > firstCount {
		t.Error("相同内容不应重复触发回调")
	}
}
