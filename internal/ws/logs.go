package ws

import (
	"log"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type LogStreamer struct {
	hub     *Hub
	logPath string
	mu      sync.Mutex
	oldLogs string
}

func NewLogStreamer(hub *Hub, logPath string) *LogStreamer {
	return &LogStreamer{
		hub:     hub,
		logPath: logPath,
	}
}

func (l *LogStreamer) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := watcher.Add(l.logPath); err != nil {
		watcher.Close()
		return err
	}

	initial, err := os.ReadFile(l.logPath)
	if err == nil {
		l.mu.Lock()
		l.oldLogs = string(initial)
		l.mu.Unlock()
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					l.processAndBroadcast()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("log watcher error: %v", err)
			}
		}
	}()

	return nil
}

func (l *LogStreamer) processAndBroadcast() {
	data, err := os.ReadFile(l.logPath)
	if err != nil {
		log.Printf("logs: read file: %v", err)
		return
	}

	l.mu.Lock()
	oldLogs := l.oldLogs
	newLogs := string(data)
	idx := len(newLogs) - len(oldLogs)
	if idx < 0 {
		idx = 0
	}
	diff := newLogs[idx:]
	l.oldLogs = newLogs
	l.mu.Unlock()

	if diff != "" {
		l.hub.Broadcast([]byte("[LOGS]" + diff))
	}
}
