package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestKeyToken_SpaceNormalization(t *testing.T) {
	// Space key maps to "space" token
	spaceMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}
	assert.Equal(t, "space", keyToken(spaceMsg))

	// Other keys pass through
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	assert.Equal(t, "j", keyToken(jMsg))

	gMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	assert.Equal(t, "g", keyToken(gMsg))
}

func TestBuildPrefixSet_TwoKeySequence(t *testing.T) {
	km := KeyMap{GoToTop: []string{"g g"}}
	km.prefixSet = buildPrefixSet(km)

	assert.True(t, km.prefixSet["g"], "should have 'g' prefix")
	assert.False(t, km.prefixSet["g g"], "should not include complete sequence")
}

func TestBuildPrefixSet_ThreeKeySequence(t *testing.T) {
	km := KeyMap{NextComment: []string{"space c j"}}
	km.prefixSet = buildPrefixSet(km)

	assert.True(t, km.prefixSet["space"], "should have 'space' prefix")
	assert.True(t, km.prefixSet["space c"], "should have 'space c' intermediate prefix")
	assert.False(t, km.prefixSet["space c j"], "should not include complete sequence")
}

func TestBuildPrefixSet_MixedSequenceLengths(t *testing.T) {
	km := KeyMap{
		GoToTop:     []string{"g g"},
		NextComment: []string{"space c j"},
		PrevComment: []string{"space c k"},
	}
	km.prefixSet = buildPrefixSet(km)

	assert.True(t, km.prefixSet["g"])
	assert.True(t, km.prefixSet["space"])
	assert.True(t, km.prefixSet["space c"])
}

func TestMatchesSequence_TwoKey(t *testing.T) {
	bindings := []string{"g j"}
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}

	assert.True(t, matchesSequence("g", jMsg, bindings))
	assert.False(t, matchesSequence("g", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}, bindings))
}

func TestMatchesSequence_ThreeKey(t *testing.T) {
	bindings := []string{"space c j"}
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}

	// After accumulating "space c", pressing j should match
	assert.True(t, matchesSequence("space c", jMsg, bindings))
	// After accumulating "space c", pressing k should not match
	assert.False(t, matchesSequence("space c", kMsg, bindings))
	// After just "space", pressing j should not match (incomplete)
	assert.False(t, matchesSequence("space", jMsg, bindings))
}

func TestMatchesSequence_SpaceKeyToken(t *testing.T) {
	bindings := []string{"space c j"}
	cMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}

	// After "space", pressing c builds "space c" — not a complete match
	assert.False(t, matchesSequence("space", cMsg, bindings))
}

func TestDefaultKeyMap_SpaceInPageDown(t *testing.T) {
	km := DefaultKeyMap()
	found := false
	for _, k := range km.PageDown {
		if k == " " {
			found = true
			break
		}
	}
	assert.True(t, found, "space should be a PageDown binding (dual-use)")
}

func TestDefaultKeyMap_SpaceIsPrefix(t *testing.T) {
	km := DefaultKeyMap()
	assert.True(t, km.prefixSet["space"], "space should be in prefix set")
	assert.True(t, km.prefixSet["space c"], "space c should be in prefix set")
}

func TestValidateBindings_NoConflicts(t *testing.T) {
	km := DefaultKeyMap()
	err := ValidateBindings(km)
	assert.NoError(t, err)
}

func TestValidateBindings_AllowsDualUse(t *testing.T) {
	// Default keymap has space in both PageDown and as a prefix — allowed via soloSet
	km := DefaultKeyMap()
	err := ValidateBindings(km)
	assert.NoError(t, err)
	assert.True(t, km.soloSet["space"], "space should be in soloSet")
}

func TestThreeKeySequence_Flow(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	// Press space — should enter pending state
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = newM.(Model)
	assert.Equal(t, "space", m.pendingKey)

	// Press c — should stay pending (intermediate prefix)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = newM.(Model)
	assert.Equal(t, "space c", m.pendingKey)

	// Press j — should clear pending (complete sequence)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(Model)
	assert.Equal(t, "", m.pendingKey)
}

func TestThreeKeySequence_CancelOnInvalidKey(t *testing.T) {
	m := makeTestModel(100)

	// Press space, then c, then x (not a valid binding)
	m = sendKey(m, " ")
	assert.Equal(t, "space", m.pendingKey)

	m = sendKey(m, "c")
	assert.Equal(t, "space c", m.pendingKey)

	m = sendKey(m, "x")
	assert.Equal(t, "", m.pendingKey, "invalid key should cancel pending state")
}

func TestDefaultKeyMap_SpaceGIsPrefix(t *testing.T) {
	km := DefaultKeyMap()
	assert.True(t, km.prefixSet["space g"], "space g should be in prefix set (for space g j / space g k)")
}

func TestMatchesSequence_SpaceGJ(t *testing.T) {
	bindings := []string{"space g j"}
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}

	assert.True(t, matchesSequence("space g", jMsg, bindings))
	assert.False(t, matchesSequence("space g", kMsg, bindings))
	assert.False(t, matchesSequence("space", jMsg, bindings))
}

func TestThreeKeySequence_SpaceGJ_Flow(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 0

	// Press space
	m = sendKey(m, " ")
	assert.Equal(t, "space", m.pendingKey)

	// Press g — should stay pending (intermediate prefix)
	m = sendKey(m, "g")
	assert.Equal(t, "space g", m.pendingKey)

	// Press j — should clear pending (complete sequence)
	m = sendKey(m, "j")
	assert.Equal(t, "", m.pendingKey)
}

func TestBuildSoloSet_SpaceIsDualUse(t *testing.T) {
	km := DefaultKeyMap()
	assert.True(t, km.soloSet["space"], "space should be in soloSet (PageDown + prefix)")
}

func TestBuildSoloSet_GIsNotDualUse(t *testing.T) {
	km := DefaultKeyMap()
	assert.False(t, km.soloSet["g"], "g should not be in soloSet (not a single-key binding)")
}

func TestBuildSoloSet_MultiTokenPrefixNotIncluded(t *testing.T) {
	km := DefaultKeyMap()
	assert.False(t, km.soloSet["space c"], "multi-token prefix should not be in soloSet")
}

func TestTwoKeySequences_StillWork(t *testing.T) {
	m := makeTestModel(100)
	m.w().scroll = 5

	// g g should still go to top
	m = sendKey(m, "g")
	assert.Equal(t, "g", m.pendingKey)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = newM.(Model)
	assert.Equal(t, "", m.pendingKey)
	assert.Equal(t, 0, m.w().scroll, "gg should scroll to top")
}
