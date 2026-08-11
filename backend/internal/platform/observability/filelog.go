package observability

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LogFileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

type LogTailResult struct {
	Lines      []string `json:"lines"`
	Offset     int64    `json:"offset"`
	NextOffset int64    `json:"next_offset"`
	EOF        bool     `json:"eof"`
}

func ListLogFiles(dir string) ([]LogFileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogFileInfo{}, nil
		}
		return nil, err
	}
	out := make([]LogFileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".log") && !strings.Contains(name, "sparkops") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, LogFileInfo{
			Name:     name,
			Size:     info.Size(),
			Modified: info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

func TailLogFile(dir, name string, offset int64, limit int) (LogTailResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	path := filepath.Join(dir, filepath.Base(name))
	f, err := os.Open(path)
	if err != nil {
		return LogTailResult{}, err
	}
	defer f.Close()

	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return LogTailResult{}, err
	}

	scanner := bufio.NewScanner(f)
	lines := make([]string, 0, limit)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= limit {
			break
		}
	}
	nextOffset, _ := f.Seek(0, 1)
	eof := nextOffset >= mustFileSize(f)
	if len(lines) < limit {
		eof = true
	}
	return LogTailResult{
		Lines:      lines,
		Offset:     offset,
		NextOffset: nextOffset,
		EOF:        eof,
	}, scanner.Err()
}

func mustFileSize(f *os.File) int64 {
	info, err := f.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}
