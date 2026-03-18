package main

import (
	"bufio"
	"encoding/json"
	"io"
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
	// allow overriding log path via env var for flexibility
	if p := os.Getenv("BEELZEBUB_LOG_PATH"); p != "" {
		logPath = p
	}

	go watchLogFile()

	http.HandleFunc("/logs", logsHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	if err := http.ListenAndServe(":8090", nil); err != nil {
		panic(err)
	}
}

func watchLogFile() {
	for {
		f, err := os.Open(logPath)
		if err != nil {
			// file may not exist yet; keep retrying
			time.Sleep(1 * time.Second)
			continue
		}

		reader := bufio.NewReader(f)
		offset := int64(0)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == os.ErrClosed {
					break
				}
				// EOF means file is still being written to; check for truncation/rotation
				if err == io.EOF {
					stat, statErr := f.Stat()
					if statErr == nil && stat.Size() < offset {
						// file rotated/truncated; reopen it
						break
					}
					time.Sleep(500 * time.Millisecond)
					continue
				}
				// unexpected error, restart watcher
				break
			}

			offset += int64(len(line))
			appendLogLine(line)
		}

		f.Close()
		// brief pause before retrying to avoid hot loop
		time.Sleep(500 * time.Millisecond)
	}
}

func appendLogLine(line string) {
	mu.Lock()
	defer mu.Unlock()
	logsBuffer = append(logsBuffer, line)
	if len(logsBuffer) > 1000 {
		logsBuffer = logsBuffer[len(logsBuffer)-1000:]
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
