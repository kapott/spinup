# tui-scaffold

Generate Bubbletea TUI component boilerplate with lipgloss styling.

## Trigger
User invokes `/tui-scaffold [component-name]` or asks to create a TUI component.

## Arguments
- `[component-name]` - Name of the component (e.g., "provider-select", "status-view")

## Instructions

1. **Create the component file** at `internal/ui/[component-name].go`

2. **Generate standard Bubbletea model**:
   ```go
   package ui

   import (
       "github.com/charmbracelet/bubbletea"
       "github.com/charmbracelet/lipgloss"
   )

   type [ComponentName]Model struct {
       // State fields
       width  int
       height int

       // Component-specific fields
   }

   func New[ComponentName]() [ComponentName]Model {
       return [ComponentName]Model{}
   }

   func (m [ComponentName]Model) Init() tea.Cmd {
       return nil
   }

   func (m [ComponentName]Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
       switch msg := msg.(type) {
       case tea.KeyMsg:
           switch msg.String() {
           case "q", "ctrl+c":
               return m, tea.Quit
           }
       case tea.WindowSizeMsg:
           m.width = msg.Width
           m.height = msg.Height
       }
       return m, nil
   }

   func (m [ComponentName]Model) View() string {
       return m.render()
   }

   func (m [ComponentName]Model) render() string {
       // Render logic here
       return ""
   }
   ```

3. **Add lipgloss styles matching PRD aesthetic**:
   ```go
   var (
       boxStyle = lipgloss.NewStyle().
           Border(lipgloss.RoundedBorder()).
           BorderForeground(lipgloss.Color("62")).
           Padding(1, 2)

       titleStyle = lipgloss.NewStyle().
           Bold(true).
           Foreground(lipgloss.Color("229"))

       selectedStyle = lipgloss.NewStyle().
           Foreground(lipgloss.Color("229")).
           Background(lipgloss.Color("57"))

       helpStyle = lipgloss.NewStyle().
           Foreground(lipgloss.Color("241"))
   )
   ```

4. **Add keyboard handling** appropriate for the component:
   - Navigation (↑↓←→, j/k, h/l)
   - Selection (Enter, Space)
   - Actions (r for refresh, s for stop, etc.)
   - Quit (q, Ctrl+C)

5. **Add help text** showing available keys

## Output Format

Creates `internal/ui/[component-name].go` with:
- Full Bubbletea model implementation
- Lipgloss styles
- Keyboard handling
- Help text rendering
