package mocks

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/mock"
)

// MockScreen is a mock implementation of tcell.Screen
type MockScreen struct {
	mock.Mock
}

// Implement all tcell.Screen methods
func (m *MockScreen) Beep() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockScreen) ChannelEvents(ch chan<- tcell.Event, quit <-chan struct{}) {
	m.Called(ch, quit)
}

func (m *MockScreen) Colors() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockScreen) EnableMouse(...tcell.MouseFlags) {
	m.Called()
}

func (m *MockScreen) DisableMouse() {
	m.Called()
}

func (m *MockScreen) EnablePaste() {
	m.Called()
}

func (m *MockScreen) DisablePaste() {
	m.Called()
}

func (m *MockScreen) HasMouse() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockScreen) HasKey(tcell.Key) bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockScreen) Sync() {
	m.Called()
}

func (m *MockScreen) CharacterSet() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockScreen) RegisterRuneFallback(r rune, subst string) {
	m.Called(r, subst)
}

func (m *MockScreen) UnregisterRuneFallback(r rune) {
	m.Called(r)
}

func (m *MockScreen) CanDisplay(r rune, checkFallbacks bool) bool {
	args := m.Called(r, checkFallbacks)
	return args.Bool(0)
}

func (m *MockScreen) Resize(int, int, int, int) {
	m.Called()
}

func (m *MockScreen) HasPendingEvent() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockScreen) Init() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockScreen) Fini() {
	m.Called()
}

func (m *MockScreen) Clear() {
	m.Called()
}

func (m *MockScreen) Show() {
	m.Called()
}

func (m *MockScreen) Size() (width, height int) {
	args := m.Called()
	return args.Int(0), args.Int(1)
}

func (m *MockScreen) PollEvent() tcell.Event {
	args := m.Called()
	if ev := args.Get(0); ev != nil {
		return ev.(tcell.Event)
	}
	return nil
}

func (m *MockScreen) PostEvent(ev tcell.Event) error {
	args := m.Called(ev)
	return args.Error(0)
}

func (m *MockScreen) SetContent(x, y int, primary rune, combining []rune, style tcell.Style) {
	m.Called(x, y, primary, combining, style)
}

func (m *MockScreen) GetContent(x, y int) (primary rune, combining []rune, style tcell.Style, width int) {
	args := m.Called(x, y)
	return args.Get(0).(rune), args.Get(1).([]rune), args.Get(2).(tcell.Style), args.Int(3)
}

// MockApplication is a mock implementation of renderer.Application
type MockApplication struct {
	mock.Mock
}

func (m *MockApplication) SetRoot(root tview.Primitive, fullscreen bool) *tview.Application {
	args := m.Called(root, fullscreen)
	if app := args.Get(0); app != nil {
		return app.(*tview.Application)
	}
	return nil
}

func (m *MockApplication) SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) *tview.Application {
	args := m.Called(capture)
	if app := args.Get(0); app != nil {
		return app.(*tview.Application)
	}
	return nil
}

func (m *MockApplication) Run() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockApplication) Stop() {
	m.Called()
}

func (m *MockApplication) Draw() {
	m.Called()
}

func (m *MockApplication) GetScreen() tcell.Screen {
	args := m.Called()
	if screen := args.Get(0); screen != nil {
		return screen.(tcell.Screen)
	}
	return nil
}
