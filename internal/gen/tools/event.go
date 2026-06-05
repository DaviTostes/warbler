package tools

import (
	"boteco/internal/db"
	"encoding/json"
	"errors"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"modernc.org/sqlite"
)

func isUniqueViolation(err error) bool {
	if sqliteErr, ok := errors.AsType[*sqlite.Error](err); ok {
		return sqliteErr.Code() == 2067 || sqliteErr.Code() == 1555
	}
	return false
}

type Event struct {
	ID          uint   `json:"id"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

func CreateEventTool(g *genkit.Genkit) *ai.ToolDef[Event, string] {
	return genkit.DefineTool(g, "create_event",
		"Create an event with description and date",
		func(ctx *ai.ToolContext, input Event) (string, error) {
			_, err := db.DB.Exec("INSERT INTO events(description, date) VALUES(?, ?)", input.Description, input.Date)
			if err != nil {
				if isUniqueViolation(err) {
					return "There's already an event marked for this date", nil
				}
				return "", err
			}

			return "Event created", nil
		},
	)
}

type DeleteEvent struct {
	ID uint `json:"id"`
}

func DeleteEventTool(g *genkit.Genkit) *ai.ToolDef[DeleteEvent, string] {
	return genkit.DefineTool(g, "delete_event",
		"Delete an event",
		func(ctx *ai.ToolContext, input DeleteEvent) (string, error) {
			_, err := db.DB.Exec("DELETE FROM events WHERE id = ?", input.ID)
			if err != nil {
				return "", err
			}

			return "Event created", nil
		},
	)
}

type FetchEventsInput struct{}

func FetchEvents(g *genkit.Genkit) *ai.ToolDef[FetchEventsInput, string] {
	return genkit.DefineTool(g, "fetch_events",
		"Fetch all Events registered",
		func(ctx *ai.ToolContext, input FetchEventsInput) (string, error) {
			rows, err := db.DB.Query("SELECT * FROM events")
			if err != nil {
				return "", err
			}
			defer rows.Close()

			events := []Event{}
			for rows.Next() {
				var e Event
				if err := rows.Scan(&e.ID, &e.Description, &e.Date); err != nil {
					return "", err
				}
				events = append(events, e)
			}

			json, err := json.Marshal(events)
			if err != nil {
				return "", err
			}

			return string(json), nil
		},
	)
}
