package promptauth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type fileSessionStore struct {
	path string
}

func newDefaultSessionStore() (SessionStore, bool, error) {
	path, err := defaultSessionFilePath()
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, false, nil
	}
	return &fileSessionStore{path: path}, true, nil
}

func (s *fileSessionStore) Load() (map[string]string, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return map[string]string{}, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}

	values := map[string]string{}
	if len(data) == 0 {
		return values, nil
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *fileSessionStore) Save(values map[string]string) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}

	data, err := json.Marshal(values)
	if err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(s.path), ".declarest-prompt-auth-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Chmod(0o600); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	if err := os.Rename(tempPath, s.path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func defaultSessionFilePath() (string, error) {
	sessionID := detectSessionID()
	if sessionID == "" {
		return "", nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	digest := sha256.Sum256([]byte(sessionID))
	fileName := "prompt-auth-" + hex.EncodeToString(digest[:8]) + ".json"
	return filepath.Join(homeDir, ".declarest", "sessions", fileName), nil
}

func detectSessionID() string {
	for _, key := range []string{
		"DECLAREST_PROMPT_AUTH_SESSION_ID",
		"TERM_SESSION_ID",
		"TMUX_PANE",
		"KITTY_WINDOW_ID",
		"WT_SESSION",
		"WINDOWID",
		"SSH_TTY",
	} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return key + ":" + value
		}
	}

	for _, file := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
		if identifier := terminalDescriptor(file); identifier != "" {
			return identifier
		}
	}

	return ""
}

func terminalDescriptor(file *os.File) string {
	if file == nil {
		return ""
	}

	info, err := file.Stat()
	if err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
		return ""
	}

	fd := strconv.FormatUint(uint64(file.Fd()), 10)
	if target, err := os.Readlink("/proc/self/fd/" + fd); err == nil && strings.TrimSpace(target) != "" {
		return "tty:" + target
	}

	name := strings.TrimSpace(file.Name())
	if name == "" {
		return ""
	}
	return "tty:" + name
}
