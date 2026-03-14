package tui

import tea "github.com/charmbracelet/bubbletea"

// Action represents a user-initiated action resolved from a key binding.
// Both single-key and sequence bindings resolve to the same Action type,
// enabling any binding to be reconfigured as either form.
type Action int

const (
	ActionNone Action = iota

	// Navigation
	ActionScrollUp
	ActionScrollDown
	ActionPageUp
	ActionPageDown
	ActionHalfUp
	ActionHalfDown
	ActionTop
	ActionGoToBottom
	ActionGoToTop
	ActionScrollLeft
	ActionScrollRight
	ActionNextHeading
	ActionPrevHeading
	ActionNextComment
	ActionPrevComment
	ActionNextAllComment
	ActionPrevAllComment
	ActionNextChange
	ActionPrevChange
	ActionNarrowNext
	ActionNarrowPrev

	// Search
	ActionSearchForward
	ActionSearchBack
	ActionNextMatch
	ActionPrevMatch
	ActionNarrowToggle

	// Folds
	ActionFoldToggle
	ActionFoldToggleAll
	ActionFullFileToggle

	// Actions
	ActionQuit
	ActionEnter
	ActionResolveToggle
	ActionYank
	ActionYankUnresolved
	ActionYankAllComments
	ActionRefreshLayout
	ActionSnapshot
	ActionSnapshotToggle
	ActionVisualMode
	ActionHelp
	ActionMoveDetect
	ActionCommentToggle
	ActionBranchFilter

	// Window management
	ActionWinSplitV
	ActionWinSplitH
	ActionWinClose
	ActionWinFocusLeft
	ActionWinFocusRight
	ActionWinFocusUp
	ActionWinFocusDown
	ActionWinResizeLeft
	ActionWinResizeRight
	ActionWinResizeUp
	ActionWinResizeDown

	// Visual mode
	ActionVisualExit
)

// actionBinding maps a KeyMap field to its Action.
type actionBinding struct {
	keys   []string
	action Action
}

// actionBindings returns the mapping from KeyMap fields to Actions.
// This is the single source of truth for all action-to-binding mappings.
func (km KeyMap) actionBindings() []actionBinding {
	return []actionBinding{
		{km.Up, ActionScrollUp},
		{km.Down, ActionScrollDown},
		{km.PageUp, ActionPageUp},
		{km.PageDown, ActionPageDown},
		{km.HalfUp, ActionHalfUp},
		{km.HalfDown, ActionHalfDown},
		{km.Top, ActionTop},
		{km.Bottom, ActionGoToBottom},
		{km.Left, ActionScrollLeft},
		{km.Right, ActionScrollRight},
		{km.GoToTop, ActionGoToTop},
		{km.NextHeading, ActionNextHeading},
		{km.PrevHeading, ActionPrevHeading},
		{km.NextComment, ActionNextComment},
		{km.PrevComment, ActionPrevComment},
		{km.NextAllComment, ActionNextAllComment},
		{km.PrevAllComment, ActionPrevAllComment},
		{km.NextChange, ActionNextChange},
		{km.PrevChange, ActionPrevChange},
		{km.NarrowNext, ActionNarrowNext},
		{km.NarrowPrev, ActionNarrowPrev},
		{km.SearchForward, ActionSearchForward},
		{km.SearchBack, ActionSearchBack},
		{km.NextMatch, ActionNextMatch},
		{km.PrevMatch, ActionPrevMatch},
		{km.NarrowToggle, ActionNarrowToggle},
		{km.FoldToggle, ActionFoldToggle},
		{km.FoldToggleAll, ActionFoldToggleAll},
		{km.FullFileToggle, ActionFullFileToggle},
		{km.Quit, ActionQuit},
		{km.Enter, ActionEnter},
		{km.ResolveToggle, ActionResolveToggle},
		{km.Yank, ActionYank},
		{km.YankUnresolved, ActionYankUnresolved},
		{km.YankAllComments, ActionYankAllComments},
		{km.RefreshLayout, ActionRefreshLayout},
		{km.Snapshot, ActionSnapshot},
		{km.SnapshotToggle, ActionSnapshotToggle},
		{km.VisualMode, ActionVisualMode},
		{km.Help, ActionHelp},
		{km.MoveDetect, ActionMoveDetect},
		{km.CommentToggle, ActionCommentToggle},
		{km.BranchFilter, ActionBranchFilter},
		{km.WinSplitV, ActionWinSplitV},
		{km.WinSplitH, ActionWinSplitH},
		{km.WinClose, ActionWinClose},
		{km.WinFocusLeft, ActionWinFocusLeft},
		{km.WinFocusRight, ActionWinFocusRight},
		{km.WinFocusUp, ActionWinFocusUp},
		{km.WinFocusDown, ActionWinFocusDown},
		{km.WinResizeLeft, ActionWinResizeLeft},
		{km.WinResizeRight, ActionWinResizeRight},
		{km.WinResizeUp, ActionWinResizeUp},
		{km.WinResizeDown, ActionWinResizeDown},
		{km.VisualExit, ActionVisualExit},
	}
}

// resolveKeyAction resolves a single-key press to an Action.
// Multi-token bindings (sequences) are skipped.
func resolveKeyAction(msg tea.KeyMsg, km KeyMap) Action {
	s := msg.String()
	for _, ab := range km.actionBindings() {
		for _, k := range ab.keys {
			if len(parseBinding(k)) > 1 {
				continue // skip sequences
			}
			if s == k {
				return ab.action
			}
		}
	}
	return ActionNone
}

// resolveSequenceAction resolves a completed key sequence to an Action.
func resolveSequenceAction(prefix string, msg tea.KeyMsg, km KeyMap) Action {
	seq := prefix + " " + keyToken(msg)
	for _, ab := range km.actionBindings() {
		for _, k := range ab.keys {
			if k == seq {
				return ab.action
			}
		}
	}
	return ActionNone
}
