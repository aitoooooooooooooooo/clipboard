package clipboard

import (
	"log/slog"
	"sync"
	"time"
)

// Watcher 剪贴板监听器
type Watcher struct {
	interval time.Duration
	lastHash string
	onChange func(*ClipboardEvent)
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewWatcher 创建监听器
// interval 为轮询间隔，建议 1000ms
func NewWatcher(interval time.Duration) *Watcher {
	return &Watcher{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// SetOnChange 设置变更回调
func (w *Watcher) SetOnChange(handler func(*ClipboardEvent)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = handler
}

// Start 开始监听
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	slog.Info("剪贴板监听器启动", "interval", w.interval)

	go w.poll()
}

// Stop 停止监听
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
	slog.Info("剪贴板监听器已停止")
}

// IsRunning 是否正在运行
func (w *Watcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// poll 轮询循环
func (w *Watcher) poll() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.check()
		}
	}
}

// check 检查剪贴板是否变化
func (w *Watcher) check() {
	data, err := readClipboard()
	if err != nil {
		slog.Debug("读取剪贴板失败", "error", err)
		return
	}

	hash := ComputeHash(data)

	w.mu.Lock()
	lastHash := w.lastHash
	w.mu.Unlock()

	if hash == lastHash {
		return
	}

	w.mu.Lock()
	w.lastHash = hash
	onChange := w.onChange
	w.mu.Unlock()

	if onChange == nil {
		return
	}

	contentType := DetectContentType(data)
	event := &ClipboardEvent{
		ContentType: contentType,
		ContentHash: hash,
		Payload:     data,
		Size:        int64(len(data)),
	}

	slog.Debug("剪贴板内容变化",
		"hash", hash,
		"type", contentType,
		"size", len(data),
	)

	onChange(event)
}
