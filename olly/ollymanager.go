package o11y

import (
	"encoding/json"
	"fmt"
	"os"
)

type O11yManager struct {
	eventChan chan AgentEvent
	logFile   *os.File
}

func NewO11yManager(logFilePath string) *O11yManager {
	file, _ := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	manager := &O11yManager{
		eventChan: make(chan AgentEvent, 1000), // 缓冲通道，防止阻塞业务
		logFile:   file,
	}

	go manager.startProcessing() // 启动后台落盘协程
	return manager
}

func (m *O11yManager) startProcessing() {
	for event := range m.eventChan {
		data, _ := json.Marshal(event)
		m.logFile.Write(append(data, '\n'))
	}
}

func (m *O11yManager) Record(e AgentEvent) {
	select {
	case m.eventChan <- e:
	default:
		fmt.Println("Warning: O11y event channel full, dropping event")
	}
}

var GlobalO11y *O11yManager
