package models

import (
	"time"
)

// StatusColumn represents a column in a kanban board (e.g., "To Do", "In Progress", "Done")
type StatusColumn struct {
    Id       int64  `json:"id" db:"id"`
    BoardId  int64  `json:"board_id" db:"board_id"`
    Name     string `json:"name" db:"name"`
    Position int    `json:"position" db:"position"` // Order of columns in the board
    Color    string `json:"color" db:"color"`       // Optional color for the column
}

// Board represents a kanban board with dynamic status columns
type Board struct {
    Id          int64          `json:"id" db:"id"`
    Title       string         `json:"title" db:"title"`
    Description string         `json:"description" db:"description"`
    CreatedAt   time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
    Columns     []StatusColumn `json:"columns" db:"-"` // Will be loaded separately
    Tasks       []Task         `json:"tasks" db:"-"`   // Will be loaded separately
}

// Task represents a task/card in a kanban board
type Task struct {
    Id             int64     `json:"id" db:"id"`
    BoardId        int64     `json:"board_id" db:"board_id"`
    StatusColumnId int64     `json:"status_column_id" db:"status_column_id"`
    title          string    `json:"title" db:"title"`
    description    string    `json:"description" db:"description"`
    Position       int       `json:"position" db:"position"` // Order within the column
    CreatedAt      time.Time `json:"created_at" db:"created_at"`
    UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`

    // Optional fields for future expansion
    Priority    int       `json:"priority" db:"priority"`         // 1=low, 2=medium, 3=high
    DueDate     *time.Time `json:"due_date" db:"due_date"`
    Assignee    string    `json:"assignee" db:"assignee"`
    Tags        string    `json:"tags" db:"tags"` // JSON array or comma-separated
}

// BubbleTea list.Item interface methods
func (t Task) Title() string {
    return t.title
}

func (t Task) Description() string {
    return t.description
}

func (t Task) FilterValue() string {
    return t.title
}

func (t *Task) SetTitle(title string) {
    t.title = title
}

func (t *Task) SetDescription(description string) {
    t.description = description
}

// NewTask creates a new Task with the given title and description
func NewTask(title, description string) Task {
    now := time.Now()
    return Task{
        title:       title,
        description: description,
        Position:    0, // Will be set when added to a column
        CreatedAt:   now,
        UpdatedAt:   now,
        Priority:    1, // Default to low priority
    }
}

// Helper methods for Board
func (b *Board) GetColumnById(columnId int64) *StatusColumn {
    for i := range b.Columns {
        if b.Columns[i].Id == columnId {
            return &b.Columns[i]
        }
    }
    return nil
}

func (b *Board) GetColumnByPosition(position int) *StatusColumn {
    for i := range b.Columns {
        if b.Columns[i].Position == position {
            return &b.Columns[i]
        }
    }
    return nil
}

// Helper methods for Task
func (t *Task) GetStatusColumn(board *Board) *StatusColumn {
    return board.GetColumnById(t.StatusColumnId)
}

func (t *Task) MoveToColumn(columnId int64) {
    t.StatusColumnId = columnId
    t.UpdatedAt = time.Now()
}

// GetTasksByColumn returns all tasks for a specific column
func (b *Board) GetTasksByColumn(columnId int64) []Task {
    var tasks []Task
    for _, task := range b.Tasks {
        if task.StatusColumnId == columnId {
            tasks = append(tasks, task)
        }
    }
    return tasks
}
