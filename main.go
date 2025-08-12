package main

import (
	"flag"
	"fmt"
	"os"

	"kanban/internal/db"
	"kanban/internal/models"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	flag.Parse()
	db, err := db.NewDB("kanban")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	m := NewModel(db)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type Mode int

const (
	Normal Mode = iota
	Insert
)

type Model struct {
	// Database connection
	db *db.TaskDB

	// Repositories (for database operations)
	boardRepo  *models.BoardRepository
	columnRepo *models.StatusColumnRepository
	taskRepo   *models.TaskRepository

	board   models.Board // Contains both Columns and Tasks
	columns []list.Model // UI components derived from board data

	// UI state
	inputPane inputPane
	width     int
	height    int
	focused   int
	mode      Mode
	err       error
}

type inputPane struct {
	titleInput       textinput.Model
	descriptionInput textinput.Model
	focused          int
}

func initInputPane() inputPane {
	titleInput := textinput.New()
	titleInput.Prompt = "Title: "
	titleInput.Placeholder = "What to do..."
	titleInput.CharLimit = 100
	titleInput.Width = 30

	descriptionInput := textinput.New()
	descriptionInput.Prompt = "Description: "
	descriptionInput.Placeholder = "Task description..."
	descriptionInput.CharLimit = 500
	descriptionInput.Width = 90

	ip := inputPane{
		titleInput:       titleInput,
		descriptionInput: descriptionInput,
		focused:          0,
	}

	return ip
}

func NewModel(database *db.TaskDB) *Model {
	// Create repositories
	boardRepo := models.NewBoardRepository(database)
	columnRepo := models.NewStatusColumnRepository(database)
	taskRepo := models.NewTaskRepository(database)

	m := &Model{
		db:         database,
		boardRepo:  boardRepo,
		columnRepo: columnRepo,
		taskRepo:   taskRepo,
		inputPane:  initInputPane(),
		focused:    0,
		mode:       Normal,
	}

	// Load or create a default board
	if err := m.loadBoard(); err != nil {
		m.err = err
	}

	// Initialize UI columns and items directly from the DB
	if m.err == nil {
		if err := m.initColumnsFromDB(); err != nil {
			m.err = err
		}
	}
	return m
}

// initColumnsFromDB builds list models for each column and loads items from the DB.
func (m *Model) initColumnsFromDB() error {
	if len(m.board.Columns) == 0 {
		return nil
	}

	m.columns = make([]list.Model, len(m.board.Columns))

	for i, column := range m.board.Columns {
		// Load tasks for this column from the DB
		tasks, err := m.taskRepo.GetByColumnId(column.Id)
		if err != nil {
			return err
		}

		// Convert to list items
		items := make([]list.Item, len(tasks))
		for j := range tasks {
			items[j] = tasks[j]
		}

		delegate := list.NewDefaultDelegate()
		c := lipgloss.Color("#325D59")
		c2 := lipgloss.Color("#325D59")
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Bold(true).Foreground(c).BorderLeftForeground(c)
		delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(c2).BorderLeftForeground(c2)

		// Size is adjusted on WindowSize messages
		lm := list.New(items, delegate, 0, 0)
		lm.Title = column.Name
		lm.Styles.Title = lm.Styles.Title.Background(c)
		lm.Styles.FilterCursor = lm.Styles.FilterCursor.Background(c)
		lm.SetShowHelp(false)

		m.columns[i] = lm
	}
	return nil
}

// rebuildColumns resizes and retitles existing column UI; it does not reload items
func (m *Model) rebuildColumns(width, height int) {
	if len(m.board.Columns) == 0 || len(m.columns) == 0 {
		return
	}

	columnWidth := width / len(m.board.Columns)
	columnHeight := height - 10 // Leave space for input and help

	for i, column := range m.board.Columns {
		if i < len(m.columns) {
			m.columns[i].Title = column.Name
			m.columns[i].SetSize(columnWidth-2, columnHeight)
		}
	}
}


// getSelectedTask returns the currently selected task in the focused column, if any
func (m *Model) getSelectedTask() (models.Task, bool) {
	if len(m.columns) == 0 || m.focused < 0 || m.focused >= len(m.columns) {
		return models.Task{}, false
	}
	item := m.columns[m.focused].SelectedItem()
	if item == nil {
		return models.Task{}, false
	}
	task, ok := item.(models.Task)
	return task, ok
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) createTask(title, description string) error {
	if len(m.board.Columns) == 0 {
		return fmt.Errorf("no columns available")
	}

	// Create task in the focused column (or first column if out of bounds)
	columnId := m.board.Columns[0].Id
	if m.focused < len(m.board.Columns) {
		columnId = m.board.Columns[m.focused].Id
	}

	task := models.NewTask(title, description)
	task.BoardId = m.board.Id
	task.StatusColumnId = columnId

	// Save to database
	if err := m.taskRepo.Create(&task); err != nil {
		return err
	}

	// Update UI only
	m.columns[m.focused].InsertItem(0, task)

	return nil
}

func handleInsert(msg tea.KeyMsg, m *Model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Exit insert mode
		m.mode = Normal
		m.inputPane.titleInput.Blur()
		m.inputPane.descriptionInput.Blur()
		return m, nil
	case "tab":
		// Switch between title and description inputs
		if m.inputPane.focused == 0 {
			m.inputPane.focused = 1
			m.inputPane.titleInput.Blur()
			return m, m.inputPane.descriptionInput.Focus()
		} else {
			m.inputPane.focused = 0
			m.inputPane.descriptionInput.Blur()
			return m, m.inputPane.titleInput.Focus()
		}
	case "enter":
		// Create the task
		title := m.inputPane.titleInput.Value()
		description := m.inputPane.descriptionInput.Value()

		if title != "" {
			if err := m.createTask(title, description); err != nil {
				m.err = err
			} else {
				// Clear inputs and exit insert mode
				m.inputPane.titleInput.SetValue("")
				m.inputPane.descriptionInput.SetValue("")
				m.inputPane.titleInput.Blur()
				m.inputPane.descriptionInput.Blur()
				m.inputPane.focused = 0
				m.mode = Normal
			}
		}
		return m, nil
	}

	// Update the focused input
	var cmd tea.Cmd
	if m.inputPane.focused == 0 {
		m.inputPane.titleInput, cmd = m.inputPane.titleInput.Update(msg)
	} else {
		m.inputPane.descriptionInput, cmd = m.inputPane.descriptionInput.Update(msg)
	}
	return m, cmd
}

func handleListInput(msg tea.KeyMsg, m *Model) (tea.Model, tea.Cmd) {
	listModel, cmd := m.columns[m.focused].Update(msg)
	m.columns[m.focused] = listModel
	return m, cmd
}

func handleNormal(msg tea.KeyMsg, m *Model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "left", "h":
		// Move focus to the left column
		if m.focused > 0 {
			m.focused--
		}
	case "right", "l":
		// Move focus to the right column
		if m.focused < len(m.columns)-1 {
			m.focused++
		}
	case "<":
		if m.focused > 0 {
			// move the selected task to the column to the left
			if task, ok := m.getSelectedTask(); ok {
				task.StatusColumnId = m.board.Columns[m.focused-1].Id
				if err := m.taskRepo.Update(&task); err != nil {
					task.StatusColumnId = m.board.Columns[m.focused].Id
					m.err = err
				} else {
					m.columns[m.focused].RemoveItem(m.columns[m.focused].Index())
					m.columns[m.focused-1].InsertItem(0, task)
					m.focused--
				}
			}
		}
	case ">":
		if m.focused < len(m.columns)-1 {
			// move the selected task to the column to the right
			if task, ok := m.getSelectedTask(); ok {
				task.StatusColumnId = m.board.Columns[m.focused+1].Id
				if err := m.taskRepo.Update(&task); err != nil {
					task.StatusColumnId = m.board.Columns[m.focused].Id
					m.err = err
				} else {
					m.columns[m.focused].RemoveItem(m.columns[m.focused].Index())
					m.columns[m.focused+1].InsertItem(0, task)
					m.focused++
				}
			}
		}

	case "d":
		if task, ok := m.getSelectedTask(); ok {
			if err := m.taskRepo.Delete(task.Id); err != nil {
				m.err = err
			} else {
				m.columns[m.focused].RemoveItem(m.columns[m.focused].Index())
			}
		}
	case "i":
		// Enter insert mode
		if !(m.columns[m.focused].SettingFilter()) {
			m.mode = Insert
			m.inputPane.focused = 0
			return m, m.inputPane.titleInput.Focus()
		}
		return handleListInput(msg, m)
	default:
		return handleListInput(msg, m)
	}
	return m, nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.rebuildColumns(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch m.mode {
		case Insert:
			return handleInsert(msg, &m)
		case Normal:
			return handleNormal(msg, &m)
		}
	}

	var cmd tea.Cmd
	if len(m.columns) > 0 && m.focused < len(m.columns) {
		// Update the focused column
		updatedList, newCmd := m.columns[m.focused].Update(msg)
		m.columns[m.focused] = updatedList
		cmd = newCmd
	}
	return m, cmd
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	if len(m.columns) == 0 {
		return "Loading..."
	}

	// Calculate the width for each column
	columnWidth := m.columns[0].Width()
	adjustedColumnWidth := columnWidth // for padding
	// Define styles for focused and unfocused columns
	focusedStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF6F59")).
		Padding(2).
		Bold(true).
		Width(adjustedColumnWidth)

	unfocusedStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#771B18")).
		Padding(2).
		Width(adjustedColumnWidth)

	column_views := make([]string, len(m.columns))
	for i, col := range m.columns {
		// Apply different styles based on focus
		if i == m.focused {
			column_views[i] = focusedStyle.Render(col.View())
		} else {
			column_views[i] = unfocusedStyle.Render(col.View())
		}
	}

	// Add a help text at the bottom
	var helpText string
	var inputPaneView string

	if m.mode == Insert {
		helpText = "\nInsert Mode: Tab to switch fields, Enter to save, Esc to cancel\n"
		inputPaneView = m.inputPane.titleInput.View() + m.inputPane.descriptionInput.View()
	} else {
		helpText = "\nPress ← → to switch columns, i to add task, d to delete task, q to quit\n"
		inputPaneView = ""
	}

	ip_style := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#FF6F59")).Width(m.width - 4)
	view := lipgloss.JoinHorizontal(lipgloss.Center, column_views...) + helpText
	if inputPaneView != "" {
		view += ip_style.Render(inputPaneView)
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, view)
}

func (m *Model) loadBoard() error {
	// Try to get the first board, or create a default one
	boards, err := m.boardRepo.GetAll()
	if err != nil {
		return err
	}

	if len(boards) == 0 {
		// Create a default board with columns
		board := &models.Board{
			Title:       "My Kanban Board",
			Description: "Default board",
		}
		if err := m.boardRepo.Create(board); err != nil {
			return err
		}

		// Create default columns
		defaultColumns := []models.StatusColumn{
			{BoardId: board.Id, Name: "To Do", Position: 0, Color: "#ff6b6b"},
			{BoardId: board.Id, Name: "In Progress", Position: 1, Color: "#4ecdc4"},
			{BoardId: board.Id, Name: "Done", Position: 2, Color: "#45b7d1"},
		}

		for _, col := range defaultColumns {
			if err := m.columnRepo.Create(&col); err != nil {
				return err
			}
		}

		// Load columns we just created into board state
		cols, err := m.columnRepo.GetByBoardId(board.Id)
		if err != nil {
			return err
		}
		board.Columns = cols
		m.board = *board
	} else {
		// Load existing board
		board, err := m.boardRepo.GetById(boards[0].Id)
		if err != nil {
			return err
		}
		m.board = *board
	}

	// Tasks are now loaded as part of the board in the repository
	return nil
}
