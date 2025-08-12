package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	gap "github.com/muesli/go-app-paths"
)

type TaskDB struct {
   db *sql.DB
	 dataDir string
}

func initDataDir(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(path, 0o770)
		}
		return err
	}
	return nil
}

func setupDataPath(name string) string {
	scope := gap.NewScope(gap.User, name)
	paths, err := scope.DataDirs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var taskDir string
	if len(paths) > 0 {
		taskDir = paths[0]
	} else {
		taskDir, _ = os.UserHomeDir()
	}

	if err := initDataDir(taskDir); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return taskDir
}

func openDB(db_path string) (*TaskDB, error) {
	db, err := sql.Open("sqlite3", db_path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &TaskDB{db, db_path}, nil
}

func NewDB(name string) (*TaskDB, error) {
	taskDir := setupDataPath(name)
	db_path := filepath.Join(taskDir, fmt.Sprintf("%s.db", name))
	db, err := openDB(db_path)
	if err != nil {
		return nil, err
	}

    // Create boards table
    sqlStmt := `
    CREATE TABLE IF NOT EXISTS boards (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        description TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `
    if _, err := db.db.Exec(sqlStmt); err != nil {
        return nil, err
    }

    // Create status_columns table
    sqlStmt = `
    CREATE TABLE IF NOT EXISTS status_columns (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        board_id INTEGER NOT NULL,
        name TEXT NOT NULL,
        position INTEGER NOT NULL,
        color TEXT DEFAULT '',
        FOREIGN KEY (board_id) REFERENCES boards(id) ON DELETE CASCADE
    );
    `
    if _, err := db.db.Exec(sqlStmt); err != nil {
        return nil, err
    }

    // Create tasks table
    sqlStmt = `
    CREATE TABLE IF NOT EXISTS tasks (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        board_id INTEGER NOT NULL,
        status_column_id INTEGER NOT NULL,
        title TEXT NOT NULL,
        description TEXT,
        position INTEGER NOT NULL DEFAULT 0,
        priority INTEGER NOT NULL DEFAULT 1,
        due_date DATETIME,
        assignee TEXT,
        tags TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (board_id) REFERENCES boards(id) ON DELETE CASCADE,
        FOREIGN KEY (status_column_id) REFERENCES status_columns(id) ON DELETE CASCADE
    );
    `
    if _, err := db.db.Exec(sqlStmt); err != nil {
        return nil, err
    }

    // Create indexes for better performance
    indexes := []string{
        "CREATE INDEX IF NOT EXISTS idx_status_columns_board_id ON status_columns(board_id);",
        "CREATE INDEX IF NOT EXISTS idx_tasks_board_id ON tasks(board_id);",
        "CREATE INDEX IF NOT EXISTS idx_tasks_status_column_id ON tasks(status_column_id);",
        "CREATE INDEX IF NOT EXISTS idx_tasks_position ON tasks(status_column_id, position);",
    }

    for _, index := range indexes {
        if _, err := db.db.Exec(index); err != nil {
            return nil, err
        }
    }

    return db, nil
}

// Implement models.DBInterface methods
func (tdb *TaskDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
    return tdb.db.Query(query, args...)
}

func (tdb *TaskDB) QueryRow(query string, args ...interface{}) *sql.Row {
    return tdb.db.QueryRow(query, args...)
}

func (tdb *TaskDB) Exec(query string, args ...interface{}) (sql.Result, error) {
    return tdb.db.Exec(query, args...)
}
