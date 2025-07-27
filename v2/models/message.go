package models

// Message represents the business logic model for our hello world display
type Message struct {
	Text string
}

// NewMessage creates a new Message
func NewMessage(text string) *Message {
	return &Message{
		Text: text,
	}
}

// GetText returns the message text
func (m *Message) GetText() string {
	return m.Text
}
