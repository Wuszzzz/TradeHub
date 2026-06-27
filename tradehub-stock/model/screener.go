package model

import "time"

type ScreeningTemplate struct {
	TemplateID     string    `json:"template_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	ConditionsJSON string    `json:"conditions_json"`
	Enabled        bool      `json:"enabled"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ScreeningResult struct {
	ResultID              string    `json:"result_id"`
	TaskID                string    `json:"task_id"`
	TemplateID            string    `json:"template_id"`
	Symbol                string    `json:"symbol"`
	Score                 float64   `json:"score"`
	MatchedConditionsJSON string    `json:"matched_conditions_json"`
	SnapshotJSON          string    `json:"snapshot_json"`
	CreatedAt             time.Time `json:"created_at"`
}
