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
)

type IdentifyRequest struct {
	ImageURL string `json:"imageUrl"`
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

func identifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IdentifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received request to identify image at URL: %s", req.ImageURL)

	imageResp, err := http.Get(req.ImageURL)
	if err != nil {
		log.Printf("Error downloading image: %v", err)
		http.Error(w, "Failed to download image", http.StatusInternalServerError)
		return
	}
	defer imageResp.Body.Close()

	imageBytes, err := io.ReadAll(imageResp.Body)
	if err != nil {
		log.Printf("Error reading image data: %v", err)
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Ajout des paramètres supplémentaires requis par iNaturalist
	if err := multipartWriter.WriteField("observation_fields", "{}"); err != nil {
		log.Printf("Error writing observation fields: %v", err)
		http.Error(w, "Failed to create form data", http.StatusInternalServerError)
		return
	}

	filePart, err := multipartWriter.CreateFormFile("image", "image.jpg")
	if err != nil {
		log.Printf("Error creating form file: %v", err)
		http.Error(w, "Failed to create form file", http.StatusInternalServerError)
		return
	}

	if _, err := filePart.Write(imageBytes); err != nil {
		log.Printf("Error writing image data to form: %v", err)
		http.Error(w, "Failed to write image data", http.StatusInternalServerError)
		return
	}

	if err := multipartWriter.Close(); err != nil {
		log.Printf("Error closing multipart writer: %v", err)
		http.Error(w, "Failed to close multipart writer", http.StatusInternalServerError)
		return
	}

	inatRequest, err := http.NewRequest("POST", "https://api.inaturalist.org/v1/computervision/score_image", &requestBody)
	if err != nil {
		log.Printf("Error creating iNaturalist request: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	inatRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	inatRequest.Header.Set("Accept", "application/json")
	inatRequest.Header.Set("User-Agent", "MushroomiNat/1.0 (champiki@example.com)")

	client := &http.Client{}
	inatResponse, err := client.Do(inatRequest)
	if err != nil {
		log.Printf("Error sending request to iNaturalist: %v", err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}
	defer inatResponse.Body.Close()

	// Lecture de la réponse pour le logging
	responseBody, err := io.ReadAll(inatResponse.Body)
	if err != nil {
		log.Printf("Error reading iNaturalist response: %v", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	// Log de la réponse pour le débogage
	log.Printf("iNaturalist response status: %d", inatResponse.StatusCode)
	log.Printf("iNaturalist response body: %s", string(responseBody))

	// Si le statut n'est pas 2xx, on retourne une erreur
	if inatResponse.StatusCode < 200 || inatResponse.StatusCode >= 300 {
		errorMsg := fmt.Sprintf("iNaturalist API error (status %d): %s", inatResponse.StatusCode, string(responseBody))
		log.Printf(errorMsg)
		http.Error(w, errorMsg, inatResponse.StatusCode)
		return
	}

	// Renvoie la réponse au client
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
