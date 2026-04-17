package models

import "time"

type Todo struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completedAt"`
	OwnerID     string     `json:"ownerId"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
