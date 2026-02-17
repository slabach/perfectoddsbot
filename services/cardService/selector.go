package cardService

import (
	"sync"
	"time"
)

const (
	selectorTTL             = 3600
	selectorCleanupInterval = 900
)

var (
	usedCardSelectors = make(map[string]int64)
	usedSelectorsMu   sync.RWMutex
)

func init() {
	go startSelectorCleanup()
}

func IsSelectorUsed(customID string) bool {
	usedSelectorsMu.RLock()
	defer usedSelectorsMu.RUnlock()
	timestamp, exists := usedCardSelectors[customID]
	if !exists {
		return false
	}
	now := time.Now().Unix()
	return (now - timestamp) < selectorTTL
}

func MarkSelectorUsed(customID string) {
	usedSelectorsMu.Lock()
	defer usedSelectorsMu.Unlock()
	usedCardSelectors[customID] = time.Now().Unix()
}

func TryMarkSelectorUsed(customID string) bool {
	usedSelectorsMu.Lock()
	defer usedSelectorsMu.Unlock()

	now := time.Now().Unix()
	timestamp, exists := usedCardSelectors[customID]

	if exists && (now-timestamp) < selectorTTL {
		return false
	}

	usedCardSelectors[customID] = now
	return true
}

func UnmarkSelectorUsed(customID string) {
	usedSelectorsMu.Lock()
	defer usedSelectorsMu.Unlock()
	delete(usedCardSelectors, customID)
}

func startSelectorCleanup() {
	ticker := time.NewTicker(time.Duration(selectorCleanupInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().Unix()
		usedSelectorsMu.Lock()
		for key, timestamp := range usedCardSelectors {
			if (now - timestamp) >= selectorTTL {
				delete(usedCardSelectors, key)
			}
		}
		usedSelectorsMu.Unlock()
	}
}
