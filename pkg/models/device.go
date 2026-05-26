package models

// Device 设备信息
type Device struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Paired      bool   `json:"paired"`
	LastSeen    int64  `json:"last_seen"`
	PublicKey   []byte `json:"public_key"`
}
