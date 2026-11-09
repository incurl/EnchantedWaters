// Package ui implements the game's user interface.
//
// # Architecture
//
// The UI is built around a stack of modal popups. Only the topmost popup
// receives the input; the popups below it stay visible but dimmed, which
// reproduces the layered dialog behaviour of the original game.
//
// The UI never mutates the simulation directly: it calls the public API
// of the world package, which validates every action.
package ui

import (
	"github.com/peter/enchantedwaters/internal/render"
)

// Popup is a modal dialog.
//
// Implementations must be idempotent in their Draw and Update methods:
// they may be drawn several times per frame.
type Popup interface {
	// Update handles the input and returns false when the popup should be
	// closed.
	Update() bool
	// Draw renders the popup.
	Draw(c *render.Canvas)
	// Title returns the title of the popup, used by the debug panel.
	Title() string
}

// PopupStack manages the stack of modal popups.
//
// The stack is intentionally simple: push, pop and peek. The manager
// guarantees that only the topmost popup is updated.
type PopupStack struct {
	// items holds the open popups, bottom first.
	items []Popup
}

// NewPopupStack creates an empty stack.
func NewPopupStack() *PopupStack {
	return &PopupStack{}
}

// Push opens a new popup on top of the stack.
func (s *PopupStack) Push(p Popup) {
	if p == nil {
		return
	}
	s.items = append(s.items, p)
}

// Pop removes the topmost popup.
//
// It reports whether a popup was removed.
func (s *PopupStack) Pop() bool {
	if len(s.items) == 0 {
		return false
	}
	s.items = s.items[:len(s.items)-1]
	return true
}

// PopAll removes every popup.
func (s *PopupStack) PopAll() {
	s.items = s.items[:0]
}

// Peek returns the topmost popup, or nil.
func (s *PopupStack) Peek() Popup {
	if len(s.items) == 0 {
		return nil
	}
	return s.items[len(s.items)-1]
}

// Len returns the number of open popups.
func (s *PopupStack) Len() int { return len(s.items) }

// Update updates the topmost popup and pops it when it asks to close.
//
// The result is true when a popup was closed during this update.
func (s *PopupStack) Update() bool {
	top := s.Peek()
	if top == nil {
		return false
	}
	if !top.Update() {
		s.Pop()
		return true
	}
	return false
}

// Draw draws every popup, from the bottom of the stack to the top.
//
// The popups below the topmost are dimmed, which gives the layered look
// of the original game's stacked dialogs.
func (s *PopupStack) Draw(canvas *render.Canvas) {
	for i, p := range s.items {
		if i < len(s.items)-1 {
			// Dim the background of the inactive popups.
			render.DimCanvas(canvas)
		}
		p.Draw(canvas)
	}
}

// Titles returns the titles of all the open popups.
func (s *PopupStack) Titles() []string {
	out := make([]string, 0, len(s.items))
	for _, p := range s.items {
		out = append(out, p.Title())
	}
	return out
}
