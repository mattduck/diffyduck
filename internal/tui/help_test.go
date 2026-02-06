package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sendKey(m Model, key string) Model {
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return newM.(Model)
}

func sendCtrl(m Model, keyType tea.KeyType) Model {
	newM, _ := m.Update(tea.KeyMsg{Type: keyType})
	return newM.(Model)
}

func TestHelp_CtrlH_TogglesHelpMode(t *testing.T) {
	m := makeTestModel(30)
	assert.False(t, m.helpMode)

	// C-h enters help mode
	m = sendCtrl(m, tea.KeyCtrlH)
	assert.True(t, m.helpMode)
	assert.Equal(t, 0, m.helpScroll)
	assert.NotEmpty(t, m.helpLines)

	// C-h again exits help mode
	m = sendCtrl(m, tea.KeyCtrlH)
	assert.False(t, m.helpMode)
}

func TestHelp_ExitKeys(t *testing.T) {
	tests := []struct {
		name    string
		keyType tea.KeyType
		runes   []rune
	}{
		{"q", tea.KeyRunes, []rune("q")},
		{"Esc", tea.KeyEsc, nil},
		{"C-g", tea.KeyCtrlG, nil},
		{"C-c", tea.KeyCtrlC, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(30)
			m = sendCtrl(m, tea.KeyCtrlH) // enter help
			require.True(t, m.helpMode)

			newM, _ := m.Update(tea.KeyMsg{Type: tt.keyType, Runes: tt.runes})
			model := newM.(Model)
			assert.False(t, model.helpMode)
		})
	}
}

func TestHelp_ScrollDown(t *testing.T) {
	m := makeTestModel(30)
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	origScroll := m.helpScroll
	m = sendKey(m, "j")
	assert.Equal(t, origScroll+1, m.helpScroll)
}

func TestHelp_ScrollUp_ClampsAtZero(t *testing.T) {
	m := makeTestModel(30)
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)
	require.Equal(t, 0, m.helpScroll)

	m = sendKey(m, "k")
	assert.Equal(t, 0, m.helpScroll, "scroll should clamp at 0")
}

func TestHelp_GG_GoesToTop(t *testing.T) {
	m := makeTestModel(30)
	m.height = 10 // small so content overflows
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	// Scroll down first
	m = sendKey(m, "j")
	m = sendKey(m, "j")
	m = sendKey(m, "j")
	require.Greater(t, m.helpScroll, 0)

	// gg goes to top
	m = sendKey(m, "g")
	m = sendKey(m, "g")
	assert.Equal(t, 0, m.helpScroll)
	assert.True(t, m.helpMode, "should still be in help mode")
}

func TestHelp_NotActiveInCommentMode(t *testing.T) {
	m := makeTestModel(30)
	m.w().commentMode = true

	// C-h in comment mode should act as backspace, not toggle help
	m = sendCtrl(m, tea.KeyCtrlH)
	assert.False(t, m.helpMode, "C-h should not open help in comment mode")
}

func TestHelp_NotActiveInSearchMode(t *testing.T) {
	m := makeTestModel(30)
	m.searchMode = true

	// C-h in search mode should not toggle help
	m = sendCtrl(m, tea.KeyCtrlH)
	assert.False(t, m.helpMode, "C-h should not open help in search mode")
}

func TestHelp_DiffScrollUnchanged(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 42

	// Enter help, scroll around, exit
	m = sendCtrl(m, tea.KeyCtrlH)
	m = sendKey(m, "j")
	m = sendKey(m, "j")
	m = sendCtrl(m, tea.KeyCtrlH)

	assert.Equal(t, 42, m.w().scroll, "diff scroll should be unchanged after help")
}

func TestHelp_ViewRendersHelp(t *testing.T) {
	m := makeTestModel(30)
	m.width = 80
	m.height = 40
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	view := m.View()
	assert.Contains(t, view, "diffyduck help")
	assert.Contains(t, view, "Navigation")
	assert.Contains(t, view, "Scroll down")
	assert.Contains(t, view, "Search")
	assert.Contains(t, view, "q, Esc, C-g or C-h to go back")
}

func TestHelp_ViewNotShownWhenOff(t *testing.T) {
	m := makeTestModel(30)
	m.width = 80
	m.height = 40

	view := m.View()
	assert.NotContains(t, view, "DiffyDuck Help")
}

// Test key formatting

func TestFormatKeyForDisplay(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"up", "↑"},
		{"down", "↓"},
		{"left", "←"},
		{"right", "→"},
		{"pgup", "PgUp"},
		{"pgdown", "PgDn"},
		{"ctrl+h", "C-h"},
		{"ctrl+c", "C-c"},
		{" ", "Space"},
		{"tab", "Tab"},
		{"shift+tab", "S-Tab"},
		{"enter", "Enter"},
		{"q", "q"},
		{"G", "G"},
		{"esc", "Esc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatKeyForDisplay(tt.input))
		})
	}
}

func TestFormatBindingKeys(t *testing.T) {
	keys := []string{"down", "j"}
	result := formatBindingKeys(keys)
	assert.Equal(t, "↓  j", result)
}

func TestAllBindingGroups_ContainsAllSections(t *testing.T) {
	km := DefaultKeyMap()
	groups := AllBindingGroups(km)

	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.Name
	}

	assert.Contains(t, names, "Navigation")
	assert.Contains(t, names, "Search")
	assert.Contains(t, names, "Folds & Files")
	assert.Contains(t, names, "Actions")
	assert.Contains(t, names, "Window Management")
	assert.Contains(t, names, "Visual Mode")
	assert.Contains(t, names, "Comment Editing")
}

func TestAllBindingGroups_GSequencesInNavigation(t *testing.T) {
	km := DefaultKeyMap()
	groups := AllBindingGroups(km)

	var navGroup BindingGroup
	for _, g := range groups {
		if g.Name == "Navigation" {
			navGroup = g
			break
		}
	}

	// gg should be standalone, gj/gk should be paired
	descs := make([]string, len(navGroup.Bindings))
	for i, b := range navGroup.Bindings {
		descs[i] = b.Desc
	}
	assert.Contains(t, descs, "Go to top")    // gg
	assert.Contains(t, descs, "Next heading") // gj (gk is paired as Desc2)

	// Check gj has gk as its pair (keys stored as sequences, not display format)
	for _, b := range navGroup.Bindings {
		if b.Desc == "Next heading" {
			assert.Equal(t, []string{"g j"}, b.Keys)
			assert.Equal(t, []string{"g k"}, b.Keys2)
			assert.Equal(t, "previous", b.Desc2)
		}
		if b.Desc == "Go to top" {
			assert.Equal(t, []string{"g g"}, b.Keys)
		}
	}
}

func TestAllBindingGroups_SearchInputMerged(t *testing.T) {
	km := DefaultKeyMap()
	groups := AllBindingGroups(km)

	var searchGroup BindingGroup
	for _, g := range groups {
		if g.Name == "Search" {
			searchGroup = g
			break
		}
	}

	descs := make([]string, len(searchGroup.Bindings))
	for i, b := range searchGroup.Bindings {
		descs[i] = b.Desc
	}
	assert.Contains(t, descs, "Search forward")
	assert.Contains(t, descs, "Execute search")   // was in SearchInput
	assert.Contains(t, descs, "Cancel search")    // was in SearchInput
	assert.Contains(t, descs, "Delete character") // was in SearchInput
}

func TestHelp_TwoColumnLayout(t *testing.T) {
	m := makeTestModel(30)
	m.width = 120 // wide enough for two columns
	m.height = 40
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	view := m.View()
	// Both Navigation (first group, left column) and a later group
	// should appear, and with two columns they can be on the same line
	assert.Contains(t, view, "Navigation")
	assert.Contains(t, view, "Comment Editing")
}

func TestHelp_SingleColumnNarrow(t *testing.T) {
	m := makeTestModel(30)
	m.width = 60 // too narrow for two columns
	m.height = 60
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	view := m.View()
	assert.Contains(t, view, "Navigation")
	assert.Contains(t, view, "Comment Editing")
}

func TestSplitBlocksIntoColumns_BalancesLineCount(t *testing.T) {
	blocks := []helpGroupBlock{
		{plain: make([]string, 10)},
		{plain: make([]string, 10)},
		{plain: make([]string, 10)},
		{plain: make([]string, 10)},
	}
	left, right := splitBlocksIntoColumns(blocks)
	// Should split roughly in half: 2 groups each (10+1+10 = 21 vs 10+1+10 = 21)
	assert.Equal(t, 2, len(left))
	assert.Equal(t, 2, len(right))
}

func TestAllBindingGroups_ModeEntryDuplicated(t *testing.T) {
	km := DefaultKeyMap()
	groups := AllBindingGroups(km)

	// V should appear in both Actions and Visual Mode
	findBinding := func(groupName, desc string) bool {
		for _, g := range groups {
			if g.Name == groupName {
				for _, b := range g.Bindings {
					if b.Desc == desc {
						return true
					}
				}
			}
		}
		return false
	}

	assert.True(t, findBinding("Actions", "Enter visual line mode"), "V should be in Actions")
	assert.True(t, findBinding("Visual Mode", "Enter visual line mode"), "V should be in Visual Mode")
	assert.True(t, findBinding("Actions", "Add comment"), "Enter should be in Actions")
	assert.True(t, findBinding("Comment Editing", "Start editing comment"), "Enter should be in Comment Editing")
}

func TestHelp_DescBeforeKeys(t *testing.T) {
	km := DefaultKeyMap()
	groups := AllBindingGroups(km)
	blocks := buildHelpGroupBlocks(groups)

	// Find the first binding line in the Navigation block (skip the header)
	navBlock := blocks[0]
	require.Greater(t, len(navBlock.plain), 1)
	line := navBlock.plain[1] // first binding line

	descIdx := strings.Index(line, "Scroll down")
	keyIdx := strings.Index(line, "j")
	require.NotEqual(t, -1, descIdx, "should contain description")
	require.NotEqual(t, -1, keyIdx, "should contain key")
	assert.Less(t, descIdx, keyIdx, "description should appear before key")
}

func TestHelp_PairedSlashInView(t *testing.T) {
	m := makeTestModel(30)
	m.width = 120
	m.height = 60
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	view := m.View()
	// Paired bindings should show "/" between desc halves and between key halves
	assert.Contains(t, view, "Scroll down")
	assert.Contains(t, view, "up") // paired opposite

	// helpLines (plain) should contain the slash separator
	found := false
	for _, line := range m.helpLines {
		if strings.Contains(line, "Scroll down") && strings.Contains(line, "/") {
			found = true
			break
		}
	}
	assert.True(t, found, "paired binding should have / separator in helpLines")
}

func TestHelp_ResizeRebuildsHelpLines(t *testing.T) {
	m := makeTestModel(30)
	m.width = 120
	m.height = 40
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	origLines := make([]string, len(m.helpLines))
	copy(origLines, m.helpLines)

	// Resize to narrow — should switch from two-column to single-column
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	m = newM.(Model)

	assert.True(t, m.helpMode, "should still be in help mode after resize")
	assert.NotEqual(t, origLines, m.helpLines, "helpLines should be rebuilt on resize")
}

func TestHelp_G_GoesToBottom(t *testing.T) {
	m := makeTestModel(30)
	m.height = 10 // small so content overflows
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)
	require.Equal(t, 0, m.helpScroll)

	m = sendKey(m, "G")
	assert.Equal(t, m.maxHelpScroll(), m.helpScroll, "G should go to bottom")
	assert.True(t, m.helpMode, "should still be in help mode")
}

func TestHelp_PageDownUp(t *testing.T) {
	m := makeTestModel(30)
	m.height = 10 // small so content overflows
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	// Page down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = newM.(Model)
	assert.Equal(t, m.helpViewportHeight(), m.helpScroll, "PgDn should scroll one viewport")

	// Page up back to top
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = newM.(Model)
	assert.Equal(t, 0, m.helpScroll, "PgUp should scroll back to top")
}

func TestHelp_HalfPageDownUp(t *testing.T) {
	m := makeTestModel(30)
	m.height = 10 // small so content overflows
	m = sendCtrl(m, tea.KeyCtrlH)
	require.True(t, m.helpMode)

	half := m.helpViewportHeight() / 2

	// Half page down (ctrl+d)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = newM.(Model)
	assert.Equal(t, half, m.helpScroll, "C-d should scroll half viewport")

	// Half page up (ctrl+u)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = newM.(Model)
	assert.Equal(t, 0, m.helpScroll, "C-u should scroll back to top")
}

func TestKeyMap_BindingGroups_MatchesActualKeys(t *testing.T) {
	km := DefaultKeyMap()
	groups := km.BindingGroups()

	// Find the Navigation group and check that it contains actual KeyMap values
	var navGroup BindingGroup
	for _, g := range groups {
		if g.Name == "Navigation" {
			navGroup = g
			break
		}
	}
	require.NotEmpty(t, navGroup.Bindings)

	// First binding should be Down with keys matching km.Down
	assert.Equal(t, km.Down, navGroup.Bindings[0].Keys)
	assert.Equal(t, "Scroll down", navGroup.Bindings[0].Desc)
}
