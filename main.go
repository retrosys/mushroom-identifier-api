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

	moRequest, err := http.NewRequest("POST", "https://mushroomobserver.org/api/v2/images/identify", &requestBody)
	if err != nil {
		log.Printf("Error creating Mushroom Observer request: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	log.Printf("Sending request to Mushroom Observer URL: %s", moRequest.URL.String())

	moRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	moRequest.Header.Set("Accept", "application/json")
	moRequest.Header.Set("User-Agent", "Mushroom Identifier/1.0")

	client := &http.Client{}
	moResponse, err := client.Do(moRequest)
	if err != nil {
		log.Printf("Error sending request to Mushroom Observer: %v", err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}
	defer moResponse.Body.Close()

	responseBody, err := io.ReadAll(moResponse.Body)
	if err != nil {
		log.Printf("Error reading Mushroom Observer response: %v", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	log.Printf("Mushroom Observer response status: %d", moResponse.StatusCode)
	log.Printf("Mushroom Observer response body: %s", string(responseBody))

	if moResponse.StatusCode < 200 || moResponse.StatusCode >= 300 {
		errorMsg := fmt.Sprintf("Mushroom Observer API error (status %d): %s", moResponse.StatusCode, string(responseBody))
		log.Printf(errorMsg)
		http.Error(w, errorMsg, moResponse.StatusCode)
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
