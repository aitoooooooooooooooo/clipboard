package storage

import (
	"testing"

	"github.com/clipboardsync/clipboardsync/pkg/models"
)

func TestSaveDevice(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	device := &models.Device{
		ID:          "device-1",
		DisplayName: "MacBook Pro",
		Paired:      true,
		LastSeen:    1000,
		PublicKey:   []byte("public-key-data"),
	}

	if err := db.SaveDevice(device); err != nil {
		t.Fatalf("SaveDevice failed: %v", err)
	}

	// 验证可以获取
	got, err := db.GetDevice("device-1")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if got.ID != device.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, device.ID)
	}
	if got.DisplayName != device.DisplayName {
		t.Errorf("DisplayName mismatch: got %s, want %s", got.DisplayName, device.DisplayName)
	}
}

func TestSaveDeviceUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	device := &models.Device{
		ID:          "device-1",
		DisplayName: "MacBook Pro",
		Paired:      false,
		LastSeen:    1000,
	}

	if err := db.SaveDevice(device); err != nil {
		t.Fatalf("SaveDevice failed: %v", err)
	}

	// 更新设备
	device.Paired = true
	device.LastSeen = 2000
	if err := db.SaveDevice(device); err != nil {
		t.Fatalf("SaveDevice update failed: %v", err)
	}

	got, err := db.GetDevice("device-1")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if !got.Paired {
		t.Error("expected device to be paired after update")
	}
	if got.LastSeen != 2000 {
		t.Errorf("expected LastSeen 2000, got %d", got.LastSeen)
	}
}

func TestListDevices(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	devices := []*models.Device{
		{ID: "d1", DisplayName: "Device 1", Paired: true, LastSeen: 1000},
		{ID: "d2", DisplayName: "Device 2", Paired: false, LastSeen: 2000},
	}

	for _, d := range devices {
		if err := db.SaveDevice(d); err != nil {
			t.Fatalf("SaveDevice failed: %v", err)
		}
	}

	got, err := db.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(got))
	}
}

func TestDeleteDevice(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	device := &models.Device{
		ID:          "device-1",
		DisplayName: "MacBook Pro",
		Paired:      true,
		LastSeen:    1000,
	}

	if err := db.SaveDevice(device); err != nil {
		t.Fatalf("SaveDevice failed: %v", err)
	}

	if err := db.DeleteDevice("device-1"); err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	// 验证已删除
	_, err := db.GetDevice("device-1")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}
