package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
)

type dartPickerModel struct {
	filepicker   filepicker.Model
	selectedFile string
	quitting     bool
	err          error
	modalUiModel
}

type clearErrorMsg struct{}

func clearErrorAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}

func NewDartPickerModel() dartPickerModel {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".yaml"}

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Find the project root by searching for the "go.mod" file
	projectRoot, err := findProjectRoot(cwd)
	if err != nil {
		panic(err)
	}
	// Construct the absolute path to the darts dir and set it as the filepicker dir
	dartsPath := filepath.Join(projectRoot, "darts")
	fp.CurrentDirectory = dartsPath

	return dartPickerModel{
		filepicker: fp,

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selectedFile: "",
		quitting:     false,
		err:          nil,
	}
}

func (m dartPickerModel) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m dartPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	case clearErrorMsg:
		m.err = nil
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		m.selectedFile = path
	}

	// Did the user select a disabled file?
	// This is only necessary to display an error to the user.
	if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
		// Let's clear the selectedFile and display an error.
		m.err = errors.New(path + " is not valid.")
		m.selectedFile = ""
		return m, tea.Batch(cmd, clearErrorAfter(2*time.Second))
	}

	return m, cmd
}

func (m dartPickerModel) View() string {
	if m.quitting {
		return ""
	}
	var s strings.Builder
	s.WriteString("\n  ")
	if m.err != nil {
		s.WriteString(m.filepicker.Styles.DisabledFile.Render(m.err.Error()))
	} else if m.selectedFile == "" {
		s.WriteString("Pick a file:")
	} else {
		s.WriteString("Selected file: " + m.filepicker.Styles.Selected.Render(m.selectedFile))
	}
	s.WriteString("\n\n" + m.filepicker.View() + "\n")
	return s.String()
}

func DartPickerTUI() {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".yaml"}

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Find the project root by searching for the "go.mod" file
	projectRoot, err := findProjectRoot(cwd)
	if err != nil {
		panic(err)
	}
	// Construct the absolute path to the darts dir and set it as the filepicker dir
	dartsPath := filepath.Join(projectRoot, "darts")
	fp.CurrentDirectory = dartsPath

	m := dartPickerModel{
		filepicker: fp,
	}
	tm, _ := tea.NewProgram(&m).Run()
	mm := tm.(dartPickerModel)
	fmt.Println("\n  You selected: " + m.filepicker.Styles.Selected.Render(mm.selectedFile) + "\n")
}

func findProjectRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			return "", fmt.Errorf("could not find project root")
		}

		dir = parentDir
	}
}
