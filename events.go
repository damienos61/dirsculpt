// events.go
// Cross-platform keyboard event management
package main

import "time"

// KeyEvent represents a keyboard input
type KeyEvent struct {
	Key       string
	Timestamp int64
}

// ProcessKeyEvent converts system key to KeyEvent
func ProcessKeyEvent(key string) KeyEvent {
	return KeyEvent{
		Key:       key,
		Timestamp: time.Now().Unix(),
	}
}
