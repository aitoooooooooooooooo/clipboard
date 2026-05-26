package models

// ClipboardEntry 剪贴板条目
type ClipboardEntry struct {
	ID           string `json:"id"`
	ContentType  string `json:"content_type"`
	ContentHash  string `json:"content_hash"`
	Payload      []byte `json:"payload"`
	SourceDevice string `json:"source_device"`
	Timestamp    int64  `json:"timestamp"`
	Size         int64  `json:"size"`
}
