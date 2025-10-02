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

var appLogo = `
 ██████╗ ███████╗████████╗    ██╗████████╗    ██████╗  ██████╗ ███╗   ██╗███████╗
██╔════╝ ██╔════╝╚══██╔══╝    ██║╚══██╔══╝    ██╔══██╗██╔═══██╗████╗  ██║██╔════╝
██║  ███╗█████╗     ██║       ██║   ██║       ██║  ██║██║   ██║██╔██╗ ██║█████╗
██║   ██║██╔══╝     ██║       ██║   ██║       ██║  ██║██║   ██║██║╚██╗██║██╔══╝
╚██████╔╝███████╗   ██║       ██║   ██║       ██████╔╝╚██████╔╝██║ ╚████║███████╗
 ╚═════╝ ╚══════╝   ╚═╝       ╚═╝   ╚═╝       ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚══════╝
`

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

// ========= STYLES SECTION =========

var (
	pineGreen   = lipgloss.Color("#325D59")
	glacierBlue = lipgloss.Color("#325D70")
	coralRed    = lipgloss.Color("#FF6F59")
	darkRed     = lipgloss.Color("#771B18")

	// Default column colors
	todoColor       = "#ff6b6b"
	inProgressColor = "#4ecdc4"
	doneColor       = "#45b7d1"
)

var (
	titlebarStyle = lipgloss.NewStyle().Foreground(coralRed)
	columnStyle   = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			Padding(1)

	unfocusedColumnStyle = columnStyle.BorderForeground(darkRed)

	focusedColumnStyle = columnStyle.
				BorderForeground(coralRed).
				Bold(true)

	inputPaneStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(coralRed)
)

func createListDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Bold(true).Foreground(pineGreen).BorderLeftForeground(pineGreen)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(glacierBlue).BorderLeftForeground(glacierBlue)
	return delegate
}

func styleListModel(lm list.Model) list.Model {
	lm.Styles.Title = lm.Styles.Title.Background(pineGreen)
	lm.Styles.FilterCursor = lm.Styles.FilterCursor.Background(pineGreen)
	lm.SetShowHelp(false)
	return lm
}

// ========= END STYLES SECTION =========

// ========= MODEL SECTION =========
type Mode int

const (
	Normal Mode = iota
	Insert
)

type Model struct {
	db *db.TaskDB

	// Repositories (for database operations)
	boardRepo  *models.BoardRepository
	columnRepo *models.StatusColumnRepository
	taskRepo   *models.TaskRepository

	board   models.Board // Contains metadata about the board (for when we have multiple boards)
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
	taskId           int64
	listIndex        int
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

	if err := m.loadBoard(); err != nil {
		m.err = err
	}

	if m.err == nil {
		if err := m.initColumnsFromDB(); err != nil {
			m.err = err
		}
	}
	return m
}

func (m *Model) initColumnsFromDB() error {
	if len(m.board.Columns) == 0 {
		return nil
	}

	m.columns = make([]list.Model, len(m.board.Columns))

	for i, column := range m.board.Columns {
		tasks, err := m.taskRepo.GetByColumnId(column.Id)
		if err != nil {
			return err
		}

		// Convert to list items
		items := make([]list.Item, len(tasks))
		for j := range tasks {
			items[j] = tasks[j]
		}

		delegate := createListDelegate()
		lm := list.New(items, delegate, 0, 0)
		lm.Title = column.Name
		lm = styleListModel(lm)

		m.columns[i] = lm
	}
	return nil
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

func (m *Model) updateTask(title, description string) error {
	if task, ok := m.getSelectedTask(); ok {
		task.SetTitle(title)
		task.SetDescription(description)
		if err := m.taskRepo.Update(&task); err != nil {
			task.StatusColumnId = m.board.Columns[m.focused].Id
			return err
		} else {
			m.columns[m.focused].SetItem(m.inputPane.listIndex, task)
		}
		m.inputPane.listIndex = -1
	}
	return nil
}

func handleInsert(msg tea.KeyMsg, m *Model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = Normal
		m.inputPane.titleInput.Blur()
		m.inputPane.descriptionInput.Blur()
		return m, nil
	case "tab":
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
		title := m.inputPane.titleInput.Value()
		description := m.inputPane.descriptionInput.Value()
		var err error
		if m.inputPane.taskId != -1 {
			err = m.updateTask(title, description)
		} else {
			if title != "" {
				err = m.createTask(title, description)
			}
		}
		if err != nil {
			m.err = err
		} else {
			m.inputPane.titleInput.SetValue("")
			m.inputPane.descriptionInput.SetValue("")
			m.inputPane.titleInput.Blur()
			m.inputPane.descriptionInput.Blur()
			m.inputPane.focused = 0
			m.mode = Normal
		}
		return m, nil
	}

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
	case "e":
		if task, ok := m.getSelectedTask(); ok {
			m.inputPane.titleInput.SetValue(task.Title())
			m.inputPane.descriptionInput.SetValue(task.Description())
			m.mode = Insert
			m.inputPane.focused = 0
			m.inputPane.taskId = task.Id
			m.inputPane.listIndex = m.columns[m.focused].Index()
			return m, m.inputPane.titleInput.Focus()
		}
	case "i":
		// Enter insert mode
		if !(m.columns[m.focused].SettingFilter()) {
			m.mode = Insert
			m.inputPane.taskId = -1
			m.inputPane.listIndex = -1
			m.inputPane.focused = 0
			return m, m.inputPane.titleInput.Focus()
		}
		return handleListInput(msg, m)
	default:
		return handleListInput(msg, m)
	}
	return m, nil
}

func (m *Model) handleWindowSize(width, height int) {
	m.width = width
	m.height = height

	if len(m.columns) == 0 {
		return
	}

	columnWidth := (m.width / len(m.columns)) - 2
	columnHeight := m.height - 17
	focusedColumnStyle = focusedColumnStyle.Width(columnWidth).Height(columnHeight)
	unfocusedColumnStyle = unfocusedColumnStyle.Width(columnWidth).Height(columnHeight)
	vertical, horizontal := columnStyle.GetFrameSize()
	inputPaneStyle = inputPaneStyle.Width(m.width - horizontal)
	for i := range m.columns {
		m.columns[i].SetSize(columnWidth-horizontal, columnHeight-vertical)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowSize(msg.Width, msg.Height)
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
		m.columns[m.focused], cmd = m.columns[m.focused].Update(msg)
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

	column_views := make([]string, len(m.columns))
	for i, col := range m.columns {
		if i == m.focused {
			column_views[i] = focusedColumnStyle.Render(col.View())
		} else {
			column_views[i] = unfocusedColumnStyle.Render(col.View())
		}
	}

	var helpText string
	var inputPaneView string

	if m.mode == Insert {
		helpText = "\nInsert Mode: Tab to switch fields, Enter to save, Esc to cancel\n"
		inputPaneView = m.inputPane.titleInput.View() + m.inputPane.descriptionInput.View()
	} else {
		helpText = "\nPress ← → to switch columns, i to add task, d to delete task, q to quit\n"
		inputPaneView = ""
	}

	titlebarView := titlebarStyle.Render(appLogo)
	boardView := lipgloss.JoinHorizontal(lipgloss.Center, column_views...) + helpText
	if inputPaneView != "" {
		inputPaneView = inputPaneStyle.Render(inputPaneView)
	}
	view := lipgloss.JoinVertical(lipgloss.Center, titlebarView, boardView, inputPaneView)
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
			{BoardId: board.Id, Name: "To Do", Position: 0, Color: todoColor},
			{BoardId: board.Id, Name: "In Progress", Position: 1, Color: inProgressColor},
			{BoardId: board.Id, Name: "Done", Position: 2, Color: doneColor},
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
