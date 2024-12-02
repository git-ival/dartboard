package tui

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rancher/dartboard/cmd/dartboard/subcommands"
	"github.com/urfave/cli/v2"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

type item struct {
	cli.Command
}

// Member functions needed for Item struct complience and for rendering the list of commands
func (i item) Title() string       { return i.Name }
func (i item) Description() string { return i.Usage }
func (i item) FilterValue() string { return i.Name }

type listKeyMap struct {
	toggleSpinner    key.Binding
	toggleTitleBar   key.Binding
	toggleStatusBar  key.Binding
	togglePagination key.Binding
	toggleHelpMenu   key.Binding
	insertItem       key.Binding
}

type rootTUIModel struct {
	dump         io.Writer
	choices      []*cli.Command   // items on the to-do list
	selected     map[int]struct{} // which to-do items are selected
	list         list.Model
	keys         *listKeyMap
	delegateKeys *delegateKeyMap
	modalUiModel
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		toggleSpinner: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle spinner"),
		),
		toggleTitleBar: key.NewBinding(
			key.WithKeys("T"),
			key.WithHelp("T", "toggle title"),
		),
		toggleStatusBar: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "toggle status"),
		),
		togglePagination: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "toggle pagination"),
		),
		toggleHelpMenu: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "toggle help"),
		),
	}
}

func TUI(c *cli.Context) error {
	var dump *os.File
	if c.Bool(subcommands.ArgsDebug) {
		var err error
		dump, err = os.OpenFile("debug-tui.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			os.Exit(1)
		}
	}

	rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

	// Runs TUI and blocks until user quits
	if _, err := tea.NewProgram(
		NewRootTUIModel(c, dump),
		// Use the full size of the terminal with its "alternate screen buffer"
		tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
	return nil
}

func NewRootTUIModel(c *cli.Context, d *os.File) rootTUIModel {
	var (
		delegateKeys = newDelegateKeyMap()
		listKeys     = newListKeyMap()
	)

	// Declare initial list of items
	var items []list.Item
	var choices []*cli.Command

	for _, command := range c.App.Commands {
		if command != nil && command.Name != "tui" {
			newItem := item{}
			newItem.Command = *command
			items = append(items, newItem)
		}
	}

	// Setup list
	delegate := newItemDelegate(delegateKeys)
	commandList := list.New(items, delegate, 0, 0)
	commandList.Title = "Commands"
	commandList.Styles.Title = titleStyle
	commandList.Paginator.Type = paginator.Arabic
	commandList.Paginator.KeyMap = paginator.KeyMap{
		PrevPage: key.NewBinding(
			key.WithKeys("pgup", "left", "h"),
			key.WithHelp("pgup / left / h", "prev page"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("pgdown", "right", "l"),
			key.WithHelp("pgdown / right / l", "next page"),
		),
	}
	commandList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			listKeys.toggleSpinner,
			listKeys.insertItem,
			listKeys.toggleTitleBar,
			listKeys.toggleStatusBar,
			listKeys.togglePagination,
			listKeys.toggleHelpMenu,
		}
	}

	return rootTUIModel{
		dump:    d,
		choices: choices,

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected:     make(map[int]struct{}),
		list:         commandList,
		keys:         listKeys,
		delegateKeys: delegateKeys,
	}
}

func (m rootTUIModel) Init() tea.Cmd {
	m.modalUiModel.modals = make(map[string]tea.Model)
	return nil
}

func (m rootTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.toggleSpinner):
			cmd := m.list.ToggleSpinner()
			return m, cmd

		case key.Matches(msg, m.keys.toggleTitleBar):
			v := !m.list.ShowTitle()
			m.list.SetShowTitle(v)
			m.list.SetShowFilter(v)
			m.list.SetFilteringEnabled(v)
			return m, nil

		case key.Matches(msg, m.keys.toggleStatusBar):
			m.list.SetShowStatusBar(!m.list.ShowStatusBar())
			return m, nil

		case key.Matches(msg, m.keys.togglePagination):
			m.list.SetShowPagination(!m.list.ShowPagination())
			return m, nil

		case key.Matches(msg, m.keys.toggleHelpMenu):
			m.list.SetShowHelp(!m.list.ShowHelp())
			return m, nil
		}
	}

	// This will also call our delegate's update function.
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m rootTUIModel) View() string {
	return appStyle.Render(m.list.View())
}
