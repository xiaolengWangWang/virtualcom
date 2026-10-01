//go:build windows

// Package virtualcom provides driverless Windows byte-stream port pairs.
// Ports are owned by a Manager; keep it alive until the application exits.
// These ports do not implement serial configuration or modem-control IOCTLs.
package virtualcom

import "github.com/xiaolengWangWang/virtualcom/internal/vcom"

type Manager = vcom.Manager
type PairInfo = vcom.PairInfo
type PortInfo = vcom.PortInfo
type PortClient = vcom.PortClient
type Snapshot = vcom.Snapshot

const (
	Version       = vcom.Version
	StateIdle     = vcom.StateIdle
	StateInUse    = vcom.StateInUse
	StateDisabled = vcom.StateDisabled
	StateError    = vcom.StateError
)

func NewManager() *Manager                      { return vcom.NewManager() }
func OpenPort(name string) (*PortClient, error) { return vcom.OpenPort(name) }
