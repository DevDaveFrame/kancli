package models

import (
	"database/sql"
	"time"
)

// DB interface for dependency injection
type DBInterface interface {
    Query(query string, args ...interface{}) (*sql.Rows, error)
    QueryRow(query string, args ...interface{}) *sql.Row
    Exec(query string, args ...interface{}) (sql.Result, error)
}

// Board CRUD operations
type BoardRepository struct {
    db DBInterface
}

func NewBoardRepository(db DBInterface) *BoardRepository {
    return &BoardRepository{db: db}
}

func (r *BoardRepository) Create(board *Board) error {
    query := `
        INSERT INTO boards (title, description, created_at, updated_at)
        VALUES (?, ?, ?, ?)
    `
    now := time.Now()
    board.CreatedAt = now
    board.UpdatedAt = now

    result, err := r.db.Exec(query, board.Title, board.Description, now, now)
    if err != nil {
        return err
    }

    id, err := result.LastInsertId()
    if err != nil {
        return err
    }

    board.Id = id
    return nil
}

func (r *BoardRepository) GetById(id int64) (*Board, error) {
    query := `
        SELECT id, title, description, created_at, updated_at
        FROM boards WHERE id = ?
    `

    board := &Board{}
    err := r.db.QueryRow(query, id).Scan(
        &board.Id, &board.Title, &board.Description,
        &board.CreatedAt, &board.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }

    // Load columns
    columnRepo := NewStatusColumnRepository(r.db)
    columns, err := columnRepo.GetByBoardId(id)
    if err != nil {
        return nil, err
    }
    board.Columns = columns

    // Load tasks
    taskRepo := NewTaskRepository(r.db)
    tasks, err := taskRepo.GetByBoardId(id)
    if err != nil {
        return nil, err
    }
    board.Tasks = tasks

    return board, nil
}

func (r *BoardRepository) GetAll() ([]Board, error) {
    query := `
        SELECT id, title, description, created_at, updated_at
        FROM boards ORDER BY created_at DESC
    `

    rows, err := r.db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var boards []Board
    for rows.Next() {
        board := Board{}
        err := rows.Scan(
            &board.Id, &board.Title, &board.Description,
            &board.CreatedAt, &board.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        boards = append(boards, board)
    }

    return boards, rows.Err()
}

func (r *BoardRepository) Update(board *Board) error {
    query := `
        UPDATE boards
        SET title = ?, description = ?, updated_at = ?
        WHERE id = ?
    `
    now := time.Now()
    board.UpdatedAt = now

    _, err := r.db.Exec(query, board.Title, board.Description, now, board.Id)
    return err
}

func (r *BoardRepository) Delete(id int64) error {
    query := `DELETE FROM boards WHERE id = ?`
    _, err := r.db.Exec(query, id)
    return err
}

// StatusColumn CRUD operations
type StatusColumnRepository struct {
    db DBInterface
}

func NewStatusColumnRepository(db DBInterface) *StatusColumnRepository {
    return &StatusColumnRepository{db: db}
}

func (r *StatusColumnRepository) Create(column *StatusColumn) error {
    query := `
        INSERT INTO status_columns (board_id, name, position, color)
        VALUES (?, ?, ?, ?)
    `

    result, err := r.db.Exec(query, column.BoardId, column.Name, column.Position, column.Color)
    if err != nil {
        return err
    }

    id, err := result.LastInsertId()
    if err != nil {
        return err
    }

    column.Id = id
    return nil
}

func (r *StatusColumnRepository) GetByBoardId(boardId int64) ([]StatusColumn, error) {
    query := `
        SELECT id, board_id, name, position, color
        FROM status_columns
        WHERE board_id = ?
        ORDER BY position
    `

    rows, err := r.db.Query(query, boardId)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var columns []StatusColumn
    for rows.Next() {
        column := StatusColumn{}
        err := rows.Scan(
            &column.Id, &column.BoardId, &column.Name,
            &column.Position, &column.Color,
        )
        if err != nil {
            return nil, err
        }
        columns = append(columns, column)
    }

    return columns, rows.Err()
}

func (r *StatusColumnRepository) Update(column *StatusColumn) error {
    query := `
        UPDATE status_columns
        SET name = ?, position = ?, color = ?
        WHERE id = ?
    `

    _, err := r.db.Exec(query, column.Name, column.Position, column.Color, column.Id)
    return err
}

func (r *StatusColumnRepository) Delete(id int64) error {
    query := `DELETE FROM status_columns WHERE id = ?`
    _, err := r.db.Exec(query, id)
    return err
}

// Task CRUD operations
type TaskRepository struct {
    db DBInterface
}

func NewTaskRepository(db DBInterface) *TaskRepository {
    return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(task *Task) error {
    query := `
        INSERT INTO tasks (board_id, status_column_id, title, description, position, priority, due_date, assignee, tags, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
    now := time.Now()
    task.CreatedAt = now
    task.UpdatedAt = now

    result, err := r.db.Exec(query,
        task.BoardId, task.StatusColumnId, task.title, task.description,
        task.Position, task.Priority, task.DueDate, task.Assignee, task.Tags,
        now, now,
    )
    if err != nil {
        return err
    }

    id, err := result.LastInsertId()
    if err != nil {
        return err
    }

    task.Id = id
    return nil
}

func (r *TaskRepository) GetByColumnId(columnId int64) ([]Task, error) {
    query := `
        SELECT id, board_id, status_column_id, title, description, position, priority, due_date, assignee, tags, created_at, updated_at
        FROM tasks
        WHERE status_column_id = ?
        ORDER BY position
    `

    rows, err := r.db.Query(query, columnId)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tasks []Task
    for rows.Next() {
        task := Task{}
        err := rows.Scan(
            &task.Id, &task.BoardId, &task.StatusColumnId,
            &task.title, &task.description, &task.Position,
            &task.Priority, &task.DueDate, &task.Assignee, &task.Tags,
            &task.CreatedAt, &task.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        tasks = append(tasks, task)
    }

    return tasks, rows.Err()
}

func (r *TaskRepository) GetByBoardId(boardId int64) ([]Task, error) {
    query := `
        SELECT id, board_id, status_column_id, title, description, position, priority, due_date, assignee, tags, created_at, updated_at
        FROM tasks
        WHERE board_id = ?
        ORDER BY status_column_id, position
    `

    rows, err := r.db.Query(query, boardId)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tasks []Task
    for rows.Next() {
        task := Task{}
        err := rows.Scan(
            &task.Id, &task.BoardId, &task.StatusColumnId,
            &task.title, &task.description, &task.Position,
            &task.Priority, &task.DueDate, &task.Assignee, &task.Tags,
            &task.CreatedAt, &task.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        tasks = append(tasks, task)
    }

    return tasks, rows.Err()
}

func (r *TaskRepository) Update(task *Task) error {
    query := `
        UPDATE tasks
        SET status_column_id = ?, title = ?, description = ?, position = ?, priority = ?, due_date = ?, assignee = ?, tags = ?, updated_at = ?
        WHERE id = ?
    `
    now := time.Now()
    task.UpdatedAt = now

    _, err := r.db.Exec(query,
        task.StatusColumnId, task.title, task.description,
        task.Position, task.Priority, task.DueDate, task.Assignee, task.Tags,
        now, task.Id,
    )
    return err
}

func (r *TaskRepository) Delete(id int64) error {
    query := `DELETE FROM tasks WHERE id = ?`
    _, err := r.db.Exec(query, id)
    return err
}

func (r *TaskRepository) MoveToColumn(taskId, columnId int64, position int) error {
    query := `
        UPDATE tasks
        SET status_column_id = ?, position = ?, updated_at = ?
        WHERE id = ?
    `
    now := time.Now()

    _, err := r.db.Exec(query, columnId, position, now, taskId)
    return err
}