package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMessage(t *testing.T) {
	text := "Hello, World!"
	msg := NewMessage(text)

	assert.NotNil(t, msg)
	assert.Equal(t, text, msg.Text)
}

func TestMessage_GetText(t *testing.T) {
	text := "Test Message"
	msg := &Message{Text: text}

	assert.Equal(t, text, msg.GetText())
}
