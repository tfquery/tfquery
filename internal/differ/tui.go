// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package differ

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hashicorp/go-tfe"
)

func SelectStateVersions(items []*tfe.StateVersion) []*tfe.StateVersion {
	p := tea.NewProgram(model{items: items})
	m, _ := p.Run()
	return m.(model).selected
}

type model struct {
	items    []*tfe.StateVersion
	cursor   int
	selected []*tfe.StateVersion
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "w":
			return m, tea.WindowSize()
		case "q", "esc":
			m.selected = nil
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ":
			if contains(m.selected, m.items[m.cursor]) {
				// Remove item from selected
				for i, v := range m.selected {
					if v.ID == m.items[m.cursor].ID {
						m.selected = append(m.selected[:i], m.selected[i+1:]...)
						break
					}
				}
			} else if len(m.selected) < 2 {
				m.selected = append(m.selected, m.items[m.cursor])
			}
		case "enter":
			if len(m.selected) == 2 {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var s strings.Builder
	s.WriteString("Select two state versions:\n\n")
	for i, sv := range m.items {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		mark := " "
		if contains(m.selected, sv) {
			mark = "x"
		}

		s.WriteString(fmt.Sprintf("%s [%s] %s %4d %s\n", cursor, mark, sv.ID, sv.Serial, sv.CreatedAt.Format("2006-01-02T15:04:05Z")))
	}
	return s.String() + "\nSPACE: toggle, ENTER: go, Q/ESCAPE: quit\n"
}

func contains(versions []*tfe.StateVersion, version *tfe.StateVersion) bool {
	for _, v := range versions {
		if v.ID == version.ID {
			return true
		}
	}
	return false
}
