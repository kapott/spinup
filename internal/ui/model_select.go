// Package ui provides the TUI (Terminal User Interface) for continueplz.
package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmeurs/continueplz/internal/models"
)

// ModelSelectModel is the Bubbletea model for the model selection view.
// It displays a table of available code-assist models, filtered by GPU compatibility.
type ModelSelectModel struct {
	// models is the list of models to display (may be filtered)
	modelList []models.Model

	// allModels contains the unfiltered list of all models
	allModels []models.Model

	// selectedGPU is the currently selected GPU for filtering (nil = show all)
	selectedGPU *models.GPU

	// cursor is the index of the currently highlighted row
	cursor int

	// selected is the index of the selected model (-1 if none)
	selected int

	// loading indicates if models are being fetched
	loading bool

	// err holds any error during model loading
	err error

	// width is the terminal width
	width int

	// height is the terminal height
	height int

	// ready indicates if the component has been initialized
	ready bool

	// focused indicates if this component has focus
	focused bool
}

// NewModelSelectModel creates a new model selection model
func NewModelSelectModel() ModelSelectModel {
	allModels := models.GetAllModels()
	m := ModelSelectModel{
		modelList: allModels,
		allModels: allModels,
		cursor:    0,
		selected:  -1,
		loading:   false,
		focused:   true,
	}
	// Sort models by tier and quality
	m.sortModels()
	return m
}

// NewModelSelectModelWithGPU creates a new model with models filtered for a specific GPU
func NewModelSelectModelWithGPU(gpu *models.GPU) ModelSelectModel {
	m := NewModelSelectModel()
	m.SetSelectedGPU(gpu)
	return m
}

// Init implements tea.Model
func (m ModelSelectModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ModelSelectModel) Update(msg tea.Msg) (ModelSelectModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case ModelsLoadedMsg:
		m.allModels = msg.Models
		m.filterModels()
		m.loading = false
		m.err = nil
		return m, nil

	case ModelsLoadErrorMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil

	case GPUSelectedMsg:
		m.SetSelectedGPU(&msg.GPU)
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input
func (m ModelSelectModel) handleKeyPress(msg tea.KeyMsg) (ModelSelectModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.modelList)-1 {
			m.cursor++
		}

	case "home":
		m.cursor = 0

	case "end":
		if len(m.modelList) > 0 {
			m.cursor = len(m.modelList) - 1
		}

	case "pgup":
		m.cursor -= 5
		if m.cursor < 0 {
			m.cursor = 0
		}

	case "pgdown":
		m.cursor += 5
		if m.cursor >= len(m.modelList) && len(m.modelList) > 0 {
			m.cursor = len(m.modelList) - 1
		}

	case "enter", " ":
		if len(m.modelList) > 0 {
			m.selected = m.cursor
			return m, func() tea.Msg {
				return ModelSelectedMsg{Model: m.modelList[m.cursor]}
			}
		}

	case "a":
		// Show all models (remove GPU filter)
		m.selectedGPU = nil
		m.filterModels()
		return m, nil

	case "r":
		// Refresh models
		m.loading = true
		return m, func() tea.Msg {
			return RefreshModelsMsg{}
		}
	}

	return m, nil
}

// View implements tea.Model
func (m ModelSelectModel) View() string {
	if !m.ready {
		return ""
	}

	return m.renderTable()
}

// renderTable renders the model selection table
func (m ModelSelectModel) renderTable() string {
	var b strings.Builder

	// Table title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	title := "Model Selection"
	if m.selectedGPU != nil {
		title = fmt.Sprintf("Model Selection (Compatible with %s)", m.selectedGPU.Name)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Handle loading state
	if m.loading {
		spinnerStyle := Styles.Spinner
		b.WriteString(spinnerStyle.Render("  Loading models..."))
		b.WriteString("\n")
		return Styles.Box.Width(m.width - 4).Render(b.String())
	}

	// Handle error state
	if m.err != nil {
		errorStyle := Styles.Error
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		b.WriteString("\n")
		b.WriteString(Styles.Muted.Render("  Press [r] to retry"))
		b.WriteString("\n")
		return Styles.Box.Width(m.width - 4).Render(b.String())
	}

	// Handle empty state
	if len(m.modelList) == 0 {
		b.WriteString(Styles.Muted.Render("  No compatible models available"))
		b.WriteString("\n")
		if m.selectedGPU != nil {
			b.WriteString(Styles.Muted.Render(fmt.Sprintf("  Selected GPU (%s) has %dGB VRAM", m.selectedGPU.Name, m.selectedGPU.VRAM)))
			b.WriteString("\n")
			b.WriteString(Styles.Muted.Render("  Press [a] to show all models"))
			b.WriteString("\n")
		}
		return Styles.Box.Width(m.width - 4).Render(b.String())
	}

	// Column widths
	colModel := 24
	colSize := 8
	colVRAM := 8
	colQuality := 12
	colGPUs := 28

	// Header
	headerStyle := Styles.TableHeader
	header := fmt.Sprintf("  %-*s %-*s %*s %-*s %-*s",
		colModel, "Model",
		colSize, "Size",
		colVRAM, "VRAM",
		colQuality, "Quality",
		colGPUs, "Compatible GPUs")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Separator
	sepLen := colModel + colSize + colVRAM + colQuality + colGPUs + 10
	b.WriteString(Styles.Muted.Render("  " + strings.Repeat(TableHorizontal, sepLen)))
	b.WriteString("\n")

	// Rows - calculate visible range for scrolling
	visibleRows := m.height - 12 // Leave room for header/footer
	if visibleRows < 5 {
		visibleRows = 5
	}
	if visibleRows > len(m.modelList) {
		visibleRows = len(m.modelList)
	}

	// Calculate start and end based on cursor position
	start := 0
	if m.cursor >= visibleRows {
		start = m.cursor - visibleRows + 1
	}
	end := start + visibleRows
	if end > len(m.modelList) {
		end = len(m.modelList)
		start = end - visibleRows
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		model := m.modelList[i]

		// Format fields
		vramStr := fmt.Sprintf("%dGB", model.VRAM)
		qualityStr := model.QualityStars()
		gpusStr := m.formatCompatibleGPUs(&model)

		// Build row
		row := fmt.Sprintf("  %-*s %-*s %*s %-*s %-*s",
			colModel, truncateString(model.Name, colModel),
			colSize, model.Params,
			colVRAM, vramStr,
			colQuality, qualityStr,
			colGPUs, truncateString(gpusStr, colGPUs))

		// Apply styling based on selection state
		if i == m.cursor {
			// Current cursor position - highlighted
			cursorStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorForeground).
				Background(ColorSelected)
			indicator := IconSelected
			if i == m.selected {
				indicator = IconCheckmark
			}
			b.WriteString(cursorStyle.Render(indicator + row[1:]))
		} else if i == m.selected {
			// Selected row
			selectedStyle := Styles.Selected
			b.WriteString(selectedStyle.Render("*" + row[1:]))
		} else {
			// Normal row - alternate colors
			if i%2 == 0 {
				b.WriteString(Styles.TableRow.Render(row))
			} else {
				b.WriteString(Styles.TableRowAlt.Render(row))
			}
		}
		b.WriteString("\n")
	}

	// Show scroll indicator if there are more items
	if len(m.modelList) > visibleRows {
		scrollInfo := fmt.Sprintf("  Showing %d-%d of %d", start+1, end, len(m.modelList))
		b.WriteString(Styles.Muted.Render(scrollInfo))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Key hints
	hints := m.renderKeyHints()
	b.WriteString(hints)

	return Styles.Box.Width(m.width - 4).Render(b.String())
}

// renderKeyHints renders the keyboard shortcut hints
func (m ModelSelectModel) renderKeyHints() string {
	hints := []string{
		FormatKeyHint("^^", "Navigate"),
		FormatKeyHint("Enter", "Select"),
	}
	if m.selectedGPU != nil {
		hints = append(hints, FormatKeyHint("a", "Show all"))
	}
	hints = append(hints, FormatKeyHint("q", "Quit"))
	return Styles.Muted.Render(strings.Join(hints, "  "))
}

// formatCompatibleGPUs returns a string of compatible GPU names for a model
func (m ModelSelectModel) formatCompatibleGPUs(model *models.Model) string {
	compatibleGPUs := models.GetCompatibleGPUs(model)
	if len(compatibleGPUs) == 0 {
		return "-"
	}

	names := make([]string, 0, len(compatibleGPUs))
	for _, gpu := range compatibleGPUs {
		names = append(names, gpu.Name)
	}
	return strings.Join(names, ", ")
}

// filterModels filters the model list based on selected GPU
func (m *ModelSelectModel) filterModels() {
	if m.selectedGPU == nil {
		m.modelList = m.allModels
	} else {
		m.modelList = models.GetCompatibleModels(m.selectedGPU.VRAM)
	}
	// Sort by tier and quality
	m.sortModels()
	// Reset cursor if it's out of bounds
	if m.cursor >= len(m.modelList) {
		if len(m.modelList) > 0 {
			m.cursor = len(m.modelList) - 1
		} else {
			m.cursor = 0
		}
	}
	// Reset selection
	m.selected = -1
}

// sortModels sorts models by tier (large first) and then by quality (highest first)
func (m *ModelSelectModel) sortModels() {
	tierOrder := map[models.Tier]int{
		models.TierLarge:  0,
		models.TierMedium: 1,
		models.TierSmall:  2,
	}

	sort.Slice(m.modelList, func(i, j int) bool {
		ti := tierOrder[m.modelList[i].Tier]
		tj := tierOrder[m.modelList[j].Tier]
		if ti != tj {
			return ti < tj
		}
		// Within same tier, sort by quality descending
		return m.modelList[i].Quality > m.modelList[j].Quality
	})
}

// truncateString truncates a string to maxLen characters, adding ellipsis if needed
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Message types for model selection

// ModelsLoadedMsg is sent when models have been loaded
type ModelsLoadedMsg struct {
	Models []models.Model
}

// ModelsLoadErrorMsg is sent when there's an error loading models
type ModelsLoadErrorMsg struct {
	Err error
}

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model models.Model
}

// RefreshModelsMsg is sent when models should be refreshed
type RefreshModelsMsg struct{}

// GPUSelectedMsg is sent when a GPU is selected (to filter models)
type GPUSelectedMsg struct {
	GPU models.GPU
}

// Getters and setters

// SetModels sets the models list
func (m *ModelSelectModel) SetModels(modelsList []models.Model) {
	m.allModels = modelsList
	m.filterModels()
	m.loading = false
}

// GetModels returns the currently displayed models
func (m ModelSelectModel) GetModels() []models.Model {
	return m.modelList
}

// GetAllModels returns all models (unfiltered)
func (m ModelSelectModel) GetAllModels() []models.Model {
	return m.allModels
}

// SetLoading sets the loading state
func (m *ModelSelectModel) SetLoading(loading bool) {
	m.loading = loading
}

// IsLoading returns true if models are being loaded
func (m ModelSelectModel) IsLoading() bool {
	return m.loading
}

// SetError sets an error
func (m *ModelSelectModel) SetError(err error) {
	m.err = err
	m.loading = false
}

// GetError returns the current error
func (m ModelSelectModel) GetError() error {
	return m.err
}

// GetCursor returns the current cursor position
func (m ModelSelectModel) GetCursor() int {
	return m.cursor
}

// SetCursor sets the cursor position
func (m *ModelSelectModel) SetCursor(cursor int) {
	if cursor >= 0 && cursor < len(m.modelList) {
		m.cursor = cursor
	}
}

// GetSelected returns the selected model index (-1 if none)
func (m ModelSelectModel) GetSelected() int {
	return m.selected
}

// GetSelectedModel returns the currently selected model, or nil if none
func (m ModelSelectModel) GetSelectedModel() *models.Model {
	if m.selected >= 0 && m.selected < len(m.modelList) {
		return &m.modelList[m.selected]
	}
	return nil
}

// GetCurrentModel returns the model at the cursor position, or nil if empty
func (m ModelSelectModel) GetCurrentModel() *models.Model {
	if len(m.modelList) > 0 && m.cursor >= 0 && m.cursor < len(m.modelList) {
		return &m.modelList[m.cursor]
	}
	return nil
}

// SetSelectedGPU sets the GPU to filter compatible models
func (m *ModelSelectModel) SetSelectedGPU(gpu *models.GPU) {
	m.selectedGPU = gpu
	m.filterModels()
}

// GetSelectedGPU returns the currently selected GPU filter
func (m ModelSelectModel) GetSelectedGPU() *models.GPU {
	return m.selectedGPU
}

// SetFocused sets whether this component has focus
func (m *ModelSelectModel) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether this component has focus
func (m ModelSelectModel) IsFocused() bool {
	return m.focused
}

// SetDimensions sets the terminal dimensions
func (m *ModelSelectModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

// Width returns the terminal width
func (m ModelSelectModel) Width() int {
	return m.width
}

// Height returns the terminal height
func (m ModelSelectModel) Height() int {
	return m.height
}

// IsReady returns whether the component has been initialized
func (m ModelSelectModel) IsReady() bool {
	return m.ready
}

// ModelCount returns the number of models currently displayed
func (m ModelSelectModel) ModelCount() int {
	return len(m.modelList)
}

// TotalModelCount returns the total number of models (unfiltered)
func (m ModelSelectModel) TotalModelCount() int {
	return len(m.allModels)
}
