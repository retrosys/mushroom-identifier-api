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
	"strings"
)

type IdentifyRequest struct {
	ImageURL string `json:"imageUrl"`
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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

	// Récupérer le token d'authentification
	authHeader := r.Header.Get("Authorization")
	// Puisque le frontend envoie déjà le token avec le préfixe JWT,
	// nous le passons directement à iNaturalist
	token := authHeader 

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

	filePart, err := multipartWriter.CreateFormFile("images", "image.jpg")
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

	inatRequest, err := http.NewRequest("POST", "https://api.inaturalist.org/v2/computervision/score_image", &requestBody)
	if err != nil {
		log.Printf("Error creating iNaturalist request: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	inatRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	inatRequest.Header.Set("Accept", "application/json")
	inatRequest.Header.Set("User-Agent", "Mushroom Identifier/1.0")
	inatRequest.Header.Set("Authorization", token) // On passe le token tel quel, avec son préfixe JWT

	client := &http.Client{}
	inatResponse, err := client.Do(inatRequest)
	if err != nil {
		log.Printf("Error sending request to iNaturalist: %v", err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}
	defer inatResponse.Body.Close()

	responseBody, err := io.ReadAll(inatResponse.Body)
	if err != nil {
		log.Printf("Error reading iNaturalist response: %v", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	log.Printf("iNaturalist response status: %d", inatResponse.StatusCode)
	log.Printf("iNaturalist response body: %s", string(responseBody))

	if inatResponse.StatusCode < 200 || inatResponse.StatusCode >= 300 {
		errorMsg := fmt.Sprintf("iNaturalist API error (status %d): %s", inatResponse.StatusCode, string(responseBody))
		log.Printf(errorMsg)
		http.Error(w, errorMsg, inatResponse.StatusCode)
		return
	}

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
