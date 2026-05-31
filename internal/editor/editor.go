package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Handler struct {
	robotsrcPath string
}

type FileEntry struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

type BlocksConfig struct {
	Requires []string          `json:"requires"`
	Header   string            `json:"header"`
	Footer   string            `json:"footer"`
	Blocks   []json.RawMessage `json:"blocks"`
}

type ProjectList struct {
	Main     string          `json:"main"`
	Blocks   BlocksConfig    `json:"blocks"`
	Projects []FileEntry     `json:"projects"`
}

func NewHandler(robotsrcPath string) *Handler {
	os.MkdirAll(robotsrcPath, 0755)

	mainPath := filepath.Join(robotsrcPath, "main.py")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		os.WriteFile(mainPath, []byte("# DO NOT DELETE\n"), 0644)
	}

	return &Handler{robotsrcPath: robotsrcPath}
}

func (h *Handler) ListFiles() (*ProjectList, error) {
	entries, err := os.ReadDir(h.robotsrcPath)
	if err != nil {
		return nil, fmt.Errorf("read robotsrc: %w", err)
	}

	result := &ProjectList{
		Main: filepath.Join(h.robotsrcPath, "main.py"),
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "main.py" || name == "blocks.json" {
			continue
		}
		if !isProjectFile(name) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(h.robotsrcPath, name))
		if err != nil {
			continue
		}
		result.Projects = append(result.Projects, FileEntry{
			Filename: name,
			Content:  string(data),
		})
	}

	blocksPath := filepath.Join(h.robotsrcPath, "blocks.json")
	if data, err := os.ReadFile(blocksPath); err == nil {
		if err := json.Unmarshal(data, &result.Blocks); err != nil {
			result.Blocks = BlocksConfig{}
		}
	}

	if result.Blocks.Requires == nil {
		result.Blocks.Requires = []string{}
	}
	if result.Blocks.Blocks == nil {
		result.Blocks.Blocks = []json.RawMessage{}
	}

	return result, nil
}

func isProjectFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".py" || ext == ".xml"
}

func (h *Handler) ReadFile(filename string) (string, error) {
	if err := validateFilename(filename); err != nil {
		return "", err
	}

	data, err := os.ReadFile(filepath.Join(h.robotsrcPath, filename))
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

func (h *Handler) WriteFile(filename string, content string) error {
	if err := validateFilename(filename); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(h.robotsrcPath, filename), []byte(content), 0644)
}

func (h *Handler) DeleteFile(filename string) error {
	if filename == "blocks.json" {
		return nil
	}
	if err := validateFilename(filename); err != nil {
		return err
	}

	return os.Remove(filepath.Join(h.robotsrcPath, filename))
}

func validateFilename(name string) error {
	if name == "" || name[0] == '.' {
		return fmt.Errorf("invalid filename")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid filename")
	}
	return nil
}
