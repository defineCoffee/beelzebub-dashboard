package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logPath    = filepath.Join("..", "logs", "beelzebub.log")
	logsBuffer = make([]string, 0, 1000) // last N entries
	mu         sync.Mutex
)

func main() {
	go watchLogFile()

	http.HandleFunc("/logs", logsHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.ListenAndServe(":8090", nil) // Change port if needed
}

func watchLogFile() {
	f, err := os.Open(logPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.Seek(0, 2) // go to end

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		mu.Lock()
		logsBuffer = append(logsBuffer, line)
		if len(logsBuffer) > 1000 {
			logsBuffer = logsBuffer[len(logsBuffer)-1000:]
		}
		mu.Unlock()
	}
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	lines := append([]string(nil), logsBuffer...)
	mu.Unlock()

	logs := make([]map[string]interface{}, 0, len(lines))
	for _, l := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(l), &obj); err == nil {
			logs = append(logs, obj)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
