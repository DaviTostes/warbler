package chat

import (
	"boteco/internal/db"
	"time"

	"github.com/firebase/genkit/go/ai"
)

func NewChat(title string) (uint, error) {
	res, err := db.DB.Exec("INSERT INTO chats(title) VALUES(?)", title)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	return uint(id), err
}

type Chat struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

func GetChats() ([]Chat, error) {
	rows, err := db.DB.Query("SELECT * FROM chats ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := []Chat{}
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}

	return chats, nil
}

func InsertMessage(chatID uint, msg *ai.Message) (uint, error) {
	res, err := db.DB.Exec(
		"INSERT INTO chat_messages(text, role, chat_id, created_at) VALUES(?, ?, ?, datetime('now'))",
		msg.Text(), msg.Role, chatID,
	)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	return uint(id), err
}

type Message struct {
	ID        uint      `json:"id"`
	Text      string    `json:"text"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func GetMessagesFromChat(chatID uint) ([]Message, error) {
	rows, err := db.DB.Query("SELECT id, text, role, created_at FROM chat_messages WHERE chat_id = ? ORDER BY id", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Text, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, nil
}
