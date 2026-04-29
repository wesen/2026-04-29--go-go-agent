package helpdb

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/help/store"
	_ "github.com/mattn/go-sqlite3"
)

// InputDBConfig describes how the embedded help database should be materialized.
type InputDBConfig struct {
	// Path is optional. If empty, a temporary SQLite file is created.
	Path string
	// HelpFS contains markdown help entries with Glazed frontmatter.
	HelpFS fs.FS
	// HelpDir is the directory inside HelpFS to load.
	HelpDir string
}

// PreparedDB is an opened SQLite handle plus metadata and cleanup.
type PreparedDB struct {
	Path    string
	DB      *sql.DB
	Cleanup func() error
}

// PrepareInputDB materializes embedded Glazed help entries into a SQLite help
// store, creates the docs compatibility view, then opens the resulting DB in
// read-only mode for the JavaScript runtime.
func PrepareInputDB(ctx context.Context, cfg InputDBConfig) (*PreparedDB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.HelpFS == nil {
		return nil, fmt.Errorf("help FS is required")
	}
	if strings.TrimSpace(cfg.HelpDir) == "" {
		return nil, fmt.Errorf("help dir is required")
	}

	path := strings.TrimSpace(cfg.Path)
	cleanup := func() error { return nil }
	if path == "" {
		tmpDir, err := os.MkdirTemp("", "chat-inputdb-*")
		if err != nil {
			return nil, err
		}
		cleanup = func() error { return os.RemoveAll(tmpDir) }
		path = filepath.Join(tmpDir, "input-help.sqlite")
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	_ = os.Remove(path)
	st, err := store.New(path)
	if err != nil {
		_ = cleanup()
		return nil, fmt.Errorf("create help store: %w", err)
	}
	hs := help.NewHelpSystemWithStore(st)
	if err := hs.LoadSectionsFromFS(cfg.HelpFS, cfg.HelpDir); err != nil {
		_ = st.Close()
		_ = cleanup()
		return nil, fmt.Errorf("load embedded help sections: %w", err)
	}
	if err := st.Close(); err != nil {
		_ = cleanup()
		return nil, err
	}

	viewDB, err := sql.Open("sqlite3", path)
	if err != nil {
		_ = cleanup()
		return nil, err
	}
	if err := ensureDocsView(ctx, viewDB); err != nil {
		_ = viewDB.Close()
		_ = cleanup()
		return nil, err
	}
	if err := viewDB.Close(); err != nil {
		_ = cleanup()
		return nil, err
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", path))
	if err != nil {
		_ = cleanup()
		return nil, err
	}
	return &PreparedDB{Path: path, DB: db, Cleanup: cleanup}, nil
}

// PrepareOutputDB creates a writable scratch SQLite database for outputDB.
func PrepareOutputDB(ctx context.Context, path string) (*PreparedDB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	path = strings.TrimSpace(path)
	cleanup := func() error { return nil }
	if path == "" {
		tmpDir, err := os.MkdirTemp("", "chat-outputdb-*")
		if err != nil {
			return nil, err
		}
		cleanup = func() error { return os.RemoveAll(tmpDir) }
		path = filepath.Join(tmpDir, "output.sqlite")
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		_ = cleanup()
		return nil, err
	}
	if err := ensureOutputSchema(ctx, db); err != nil {
		_ = db.Close()
		_ = cleanup()
		return nil, err
	}
	return &PreparedDB{Path: path, DB: db, Cleanup: cleanup}, nil
}

func ensureOutputSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key TEXT,
  value TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`)
	return err
}

func ensureDocsView(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE VIEW IF NOT EXISTS docs AS
SELECT
  id,
  slug,
  section_type,
  title,
  sub_title,
  short,
  content,
  topics,
  flags,
  commands,
  is_top_level,
  is_template,
  show_per_default,
  order_num,
  created_at,
  updated_at
FROM sections;`)
	return err
}

func (p *PreparedDB) Close() error {
	if p == nil {
		return nil
	}
	var first error
	if p.DB != nil {
		first = p.DB.Close()
	}
	if p.Cleanup != nil {
		if err := p.Cleanup(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
