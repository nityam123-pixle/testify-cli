package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Timestamp    time.Time
	Method       string
	Path         string
	Status       int
	Duration     time.Duration
	RequestBody  string
	ResponseBody string
}

func Save(projectDir string, entry Entry) error {
	dir := filepath.Join(projectDir, ".testify")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(filepath.Join(dir, "history.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = file.Write(append(b, '\n'))
	return err
}

func Load(projectDir string) ([]Entry, error) {
	file, err := os.Open(filepath.Join(projectDir, ".testify", "history.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	// Buffer size might need to be increased if body is large, but default 64K is okay for now.
	// Actually, let's bump it up just in case response body is large.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			entries = append(entries, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
