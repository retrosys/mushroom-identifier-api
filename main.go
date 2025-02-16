package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type IdentifyRequest struct {
	ImageURL string `json:"imageUrl"`
	APIKey   string `json:"apiKey"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func sendErrorResponse(w http.ResponseWriter, status int, message string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := ErrorResponse{
		Error:   message,
		Details: details,
	}
	json.NewEncoder(w).Encode(response)
}

func identifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST requests are accepted")
		return
	}

	var req IdentifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.ImageURL == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Missing image URL", "The imageUrl field is required")
		return
	}

	if req.APIKey == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Missing API key", "The apiKey field is required")
		return
	}

	log.Printf("Processing identification request for image: %s", req.ImageURL)

	// Client HTTP avec timeout plus long pour le téléchargement
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Télécharger l'image
	log.Printf("Downloading image from URL: %s", req.ImageURL)
	imageResp, err := client.Get(req.ImageURL)
	if err != nil {
		log.Printf("Error downloading image: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to download image", err.Error())
		return
	}
	defer imageResp.Body.Close()

	if imageResp.StatusCode != http.StatusOK {
		log.Printf("Error downloading image. Status: %d", imageResp.StatusCode)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to download image", fmt.Sprintf("Image server returned status: %d", imageResp.StatusCode))
		return
	}

	// Lire l'image
	imageBytes, err := io.ReadAll(imageResp.Body)
	if err != nil {
		log.Printf("Error reading image data: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to read image data", err.Error())
		return
	}

	log.Printf("Successfully read image data. Size: %d bytes", len(imageBytes))

	// Préparer la requête multipart
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Ajouter la clé API
	if err := multipartWriter.WriteField("api_key", req.APIKey); err != nil {
		log.Printf("Error writing api_key field: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to prepare request", err.Error())
		return
	}

	// Ajouter la méthode
	if err := multipartWriter.WriteField("method", "identify_image"); err != nil {
		log.Printf("Error writing method field: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to prepare request", err.Error())
		return
	}

	// Ajouter l'image
	filePart, err := multipartWriter.CreateFormFile("file", "image.jpg")
	if err != nil {
		log.Printf("Error creating form file: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to prepare image upload", err.Error())
		return
	}

	if _, err := filePart.Write(imageBytes); err != nil {
		log.Printf("Error writing image data: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to process image data", err.Error())
		return
	}

	if err := multipartWriter.Close(); err != nil {
		log.Printf("Error closing multipart writer: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to finalize request", err.Error())
		return
	}

	// Préparer la requête à Mushroom Observer
	moURL := "https://mushroomobserver.org/api2"
	log.Printf("Sending request to Mushroom Observer API: %s", moURL)

	moRequest, err := http.NewRequest("POST", moURL, &requestBody)
	if err != nil {
		log.Printf("Error creating MO request: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to create identification request", err.Error())
		return
	}

	moRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	moRequest.Header.Set("Accept", "application/json")
	moRequest.Header.Set("User-Agent", "Mushroom Identifier/1.0")

	// Envoyer la requête avec un timeout plus long
	moClient := &http.Client{
		Timeout: 90 * time.Second,
	}

	log.Printf("Sending request to MO with Content-Type: %s", moRequest.Header.Get("Content-Type"))
	moResponse, err := moClient.Do(moRequest)
	if err != nil {
		log.Printf("Error sending request to MO: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to connect to identification service", err.Error())
		return
	}
	defer moResponse.Body.Close()

	// Lire la réponse
	responseBody, err := io.ReadAll(moResponse.Body)
	if err != nil {
		log.Printf("Error reading MO response: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to read service response", err.Error())
		return
	}

	log.Printf("MO response status: %d", moResponse.StatusCode)
	log.Printf("MO response body: %s", string(responseBody))

	// Vérifier le statut de la réponse
	if moResponse.StatusCode < 200 || moResponse.StatusCode >= 300 {
		errorMsg := fmt.Sprintf("Mushroom Observer API error (status %d): %s", moResponse.StatusCode, string(responseBody))
		log.Printf(errorMsg)
		sendErrorResponse(w, moResponse.StatusCode, "Identification service error", string(responseBody))
		return
	}

	// Renvoyer la réponse au client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/identify", enableCORS(identifyHandler))

	log.Printf("Server started on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
