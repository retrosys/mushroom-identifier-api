package models

type IdentifyRequest struct {
    ImageURL string `json:"imageUrl"`
    APIKey   string `json:"apiKey"`
}

type ErrorResponse struct {
    Error   string `json:"error"`
    Details string `json:"details,omitempty"`
}
