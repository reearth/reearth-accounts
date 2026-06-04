package internal

// ErrorResponse is the standard JSON error body returned by the admin API
// error handler.
type ErrorResponse struct {
	Code    string `json:"code" example:"Bad Request"`
	Message string `json:"message" example:"invalid request"`
}
