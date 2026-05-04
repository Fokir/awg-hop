package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"awghop/internal/db"
)

// ReplaceDatabase атомарно заменяет файл базы (с резервной копией .bak)
// и переоткрывает соединение через db.Open. После успешной замены сессии
// в новой БД сохраняются — пользователь автоматически перелогинится при
// следующем запросе.
func (s *Store) ReplaceDatabase(ctx context.Context, dbPath string, contents []byte) error {
	if len(contents) < 16 {
		return fmt.Errorf("backup database too small: %d bytes", len(contents))
	}

	if s.db != nil {
		_ = s.db.Close()
	}

	tmp := dbPath + ".tmp"
	if err := os.WriteFile(tmp, contents, 0o600); err != nil {
		return fmt.Errorf("write tmp db: %w", err)
	}

	bak := dbPath + ".bak"
	if _, err := os.Stat(dbPath); err == nil {
		_ = os.Remove(bak)
		if err := os.Rename(dbPath, bak); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("backup current db: %w", err)
		}
	}

	if err := os.Rename(tmp, dbPath); err != nil {
		// попытка откатить .bak обратно
		if _, statErr := os.Stat(bak); statErr == nil {
			_ = os.Rename(bak, dbPath)
		}
		return fmt.Errorf("install new db: %w", err)
	}

	newDB, err := db.Open(dbPath)
	if err != nil {
		// откатываемся к бэкапу
		_ = os.Remove(dbPath)
		if _, statErr := os.Stat(bak); statErr == nil {
			_ = os.Rename(bak, dbPath)
			if reopened, openErr := db.Open(dbPath); openErr == nil {
				s.db = reopened
			}
		}
		return fmt.Errorf("open new db: %w", err)
	}

	s.db = newDB

	// Гарантируем, что директория с БД пишется на диск.
	if dir := filepath.Dir(dbPath); dir != "" {
		if d, err := os.Open(dir); err == nil {
			_ = d.Sync()
			_ = d.Close()
		}
	}
	return nil
}
