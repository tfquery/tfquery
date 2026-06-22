// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/apex/log"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/command/si"
	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/meta"
	"github.com/tfquery/tfquery/internal/state"
)

func siCommandAction(ctx context.Context, cmd *cli.Command) error {
	// SiCommandAction is the action handler for the "si" subcommand. It
	// loads Terraform state for the target root directory and launches an
	// interactive inspector UI to explore resources and outputs.
	meta := cmd.Metadata["meta"].(meta.Meta)
	log.Debugf("Executing action for %v", meta.Args[1:])

	config.Config.Namespace = "si"

	// Use the same backend detection and state loading as sq
	stateData, err := state.LoadStateData(ctx, cmd, meta.RootDir)
	if err != nil {
		return err
	}

	// Run interactive console
	return runSiInteractiveConsole(stateData)
}

// siModel represents the Bubble Tea model for si command
type siModel struct {
	input          textinput.Model
	history        []string // Full history for navigation (includes file history)
	sessionHistory []string // Only commands from this session (matches with outputs)
	histIndex      int
	output         []string
	stateData      map[string]interface{}
}

func initialSiModel(stateData map[string]interface{}) siModel {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 2048
	ti.Width = 999
	ti.Prompt = ""
	ti.Cursor.SetMode(cursor.CursorBlink) // Set to blinking vertical line

	// Load history from file
	history := loadSiHistory(getSiHistoryFile())

	// Add initial welcome message
	var output []string
	resources, ok := stateData["resources"].([]interface{})
	if ok {
		output = append(output, fmt.Sprintf("Interactive state console loaded. %d resources found.", len(resources)))
	}
	output = append(output, "Type 'help' for syntax, 'exit' or Ctrl+C to quit.")

	return siModel{
		input:          ti,
		history:        history,
		sessionHistory: []string{}, // Empty for new session
		histIndex:      -1,
		output:         output,
		stateData:      stateData,
	}
}

func (m siModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m siModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			entry := m.input.Value()
			if strings.TrimSpace(entry) != "" {
				// Handle special commands
				if entry == "exit" || entry == "quit" {
					return m, tea.Quit
				}
				if entry == "help" {
					m.history = append(m.history, entry)
					m.sessionHistory = append(m.sessionHistory, entry)
					m.histIndex = -1
					m.output = append(m.output, getSiHelp())
					saveSiHistory(getSiHistoryFile(), m.history)
					m.input.SetValue("")
					return m, nil
				}

				// Process query and get output
				result := processSiQuery(m.stateData, entry)

				m.history = append(m.history, entry)
				m.sessionHistory = append(m.sessionHistory, entry)
				m.histIndex = -1
				m.output = append(m.output, result)
				saveSiHistory(getSiHistoryFile(), m.history)
			}
			m.input.SetValue("")
			return m, nil

		case "up":
			if len(m.history) == 0 {
				return m, nil
			}
			if m.histIndex == -1 {
				m.histIndex = len(m.history) - 1
			} else if m.histIndex > 0 {
				m.histIndex--
			}
			m.input.SetValue(m.history[m.histIndex])
			m.input.CursorEnd()
			return m, nil

		case "down":
			if len(m.history) == 0 {
				return m, nil
			}
			if m.histIndex >= 0 && m.histIndex < len(m.history)-1 {
				m.histIndex++
				m.input.SetValue(m.history[m.histIndex])
				m.input.CursorEnd()
			} else {
				m.histIndex = -1
				m.input.SetValue("")
			}
			return m, nil

		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m siModel) View() string {
	// Terraform purple style for the prompt
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#623CE4"))

	var lines []string

	// Add the initial welcome messages first
	if len(m.output) >= 2 {
		lines = append(lines, m.output[0])
		lines = append(lines, m.output[1])
	}

	// Add each command from THIS SESSION with its corresponding output
	for i := 0; i < len(m.sessionHistory); i++ {
		// Show the command that was entered in this session
		lines = append(lines, promptStyle.Render("> ")+m.sessionHistory[i])

		// Show the corresponding output (accounting for the 2 initial messages)
		if (i + 2) < len(m.output) {
			lines = append(lines, m.output[i+2])
		}
	}

	// Add current prompt and input
	lines = append(lines, promptStyle.Render("> ")+m.input.View())

	return strings.Join(lines, "\n")
}

// getSiHelp returns the help text as a string
func getSiHelp() string {
	return `Query syntax:
  Three query modes supported:

  1. JSON output (queries starting with '.')
     .module.sample                    - All resources in module.sample
     .module.sample.xxx.data          - All data sources in module.sample.xxx
     .module.sample.random_id.uuid    - Specific resource as JSON
     .module.sample.aws_security_group[3]        - Indexed resource
     .module.sample.aws_security_group["primary"] - Named resource

  2. List output (queries not starting with '.')
     module.sample                    - List all resources in module.sample
     module.sample.aws_instance       - List all aws_instance resources
     module.sample.aws_instance.web   - List specific resource
     module.sample.aws_security_group[3]        - List indexed resource
     module.sample.aws_security_group["primary"] - List named resource

  3. Function evaluation (queries starting with '/')
     /coalesce(null, "default")       - Evaluate coalesce function
     /length("hello")                 - Get string length
     /upper("world")                  - Convert to uppercase
     /keys(outputs)                   - List output keys

  Special queries:
     terraform_version                - Get Terraform version
     version                          - Get state file version
     outputs.name                     - Get output value

  Navigation:
     ↑/↓ arrows                       - Navigate command history
     Ctrl+C                           - Exit

  Examples:
     .aws_instance.web[0]             - JSON for first aws_instance.web
     /coalesce(null, "fallback")      - Function evaluation`
}

// getSiHistoryFile returns the path to the si history file
func getSiHistoryFile() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".tfctl_si_history"
	}
	return filepath.Join(homeDir, ".tfctl_si_history")
}

func loadSiHistory(filename string) []string {
	var history []string

	file, err := os.Open(filename)
	if err != nil {
		return history // Return empty history if file doesn't exist
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			history = append(history, line)
		}
	}

	return history
}

func processSiQuery(stateData map[string]interface{}, query string) string {
	var result strings.Builder

	// Capture fmt.Print output by temporarily redirecting
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Process the query (this will write to our pipe instead of stdout)
	si.ProcessQuery(stateData, query)

	// Restore stdout and read what was written
	w.Close()
	os.Stdout = oldStdout

	// Read the captured output
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			result.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	r.Close()

	output := result.String()
	if output == "" {
		return "No results found."
	}
	return strings.TrimSuffix(output, "\n")
}

func runSiInteractiveConsole(stateData map[string]interface{}) error {
	p := tea.NewProgram(initialSiModel(stateData))
	_, err := p.Run()
	return err
}

func saveSiHistory(filename string, history []string) {
	// Keep only the last 1000 commands
	maxHistory := 1000
	start := 0
	if len(history) > maxHistory {
		start = len(history) - maxHistory
	}

	file, err := os.Create(filename)
	if err != nil {
		return // Silently fail if we can't save history
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for i := start; i < len(history); i++ {
		fmt.Fprintln(writer, history[i])
	}
	writer.Flush()
}

// SiCommandBuilder constructs the cli.Command for "si" and wires up metadata,
// flags, and the action handler.
func siCommandBuilder(meta meta.Meta) *cli.Command {
	return &cli.Command{
		Name:      "si",
		Hidden:    true,
		Usage:     "state inspector",
		UsageText: "tfquery si [RootDir] [options]",
		Metadata: map[string]any{
			"meta": meta,
		},
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:    "passphrase",
				Aliases: []string{"p"},
				Usage:   "passphrase for encrypted state files",
				Value:   "",
			},
			&cli.StringFlag{
				Name:        "sv",
				Usage:       "state version to query",
				Value:       "0",
				HideDefault: true,
			},
		}, NewGlobalFlags("si")...,
		),
		Action: siCommandAction,
	}
}
