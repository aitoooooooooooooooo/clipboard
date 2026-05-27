package storage

import (
	"fmt"

	"github.com/clipboardsync/clipboardsync/pkg/models"
)

// SaveDevice 保存或更新设备信息
func (db *DB) SaveDevice(device *models.Device) error {
	query := `INSERT OR REPLACE INTO devices (id, display_name, paired, last_seen, public_key)
			  VALUES (?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query,
		device.ID,
		device.DisplayName,
		device.Paired,
		device.LastSeen,
		device.PublicKey,
	)
	if err != nil {
		return fmt.Errorf("failed to save device: %w", err)
	}
	return nil
}

// GetDevice 根据 ID 获取设备
func (db *DB) GetDevice(id string) (*models.Device, error) {
	query := `SELECT id, display_name, paired, last_seen, public_key
			  FROM devices WHERE id = ?`
	device := &models.Device{}
	err := db.conn.QueryRow(query, id).Scan(
		&device.ID,
		&device.DisplayName,
		&device.Paired,
		&device.LastSeen,
		&device.PublicKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}
	return device, nil
}

// ListDevices 获取所有设备
func (db *DB) ListDevices() ([]*models.Device, error) {
	query := `SELECT id, display_name, paired, last_seen, public_key FROM devices`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		device := &models.Device{}
		if err := rows.Scan(
			&device.ID,
			&device.DisplayName,
			&device.Paired,
			&device.LastSeen,
			&device.PublicKey,
		); err != nil {
			return nil, fmt.Errorf("failed to scan device: %w", err)
		}
		devices = append(devices, device)
	}
	return devices, nil
}

// DeleteDevice 删除设备
func (db *DB) DeleteDevice(id string) error {
	query := `DELETE FROM devices WHERE id = ?`
	result, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("device not found: %s", id)
	}
	return nil
}
