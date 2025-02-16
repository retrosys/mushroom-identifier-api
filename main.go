package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/retrosys/mushroom-identifier-api/handlers"
	"github.com/retrosys/mushroom-identifier-api/models"
	"github.com/retrosys/mushroom-identifier-api/services"
	"github.com/retrosys/mushroom-identifier-api/utils"
)

func sendErrorResponse(w http.ResponseWriter, status int, message string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   message,
		Details: details,
	})
}

func identifyHandler(w http.ResponseWriter, r *http.Request) {
	moService := services.NewMushroomObserverService()

	if err := moService.CheckAvailability(); err != nil {
		log.Printf("Server availability check failed: %v", err)
		sendErrorResponse(w, http.StatusServiceUnavailable,
			"Le service d'identification n'est pas disponible pour le moment",
			"Veuillez réessayer dans quelques minutes")
		return
	}

	if r.Method != "POST" {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST requests are accepted")
		return
	}

	var req models.IdentifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.ImageURL == "" || req.APIKey == "" {
		sendErrorResponse(w, http.StatusBadRequest, "Missing required fields", "Both imageUrl and apiKey are required")
		return
	}

	log.Printf("Processing identification request for image: %s", req.ImageURL)

	// Download the image
	client := utils.NewHTTPClient(180 * time.Second)
	imageResp, err := client.Get(req.ImageURL)
	if err != nil {
		log.Printf("Error downloading image: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to download image", err.Error())
		return
	}
	defer imageResp.Body.Close()

	if imageResp.StatusCode != http.StatusOK {
		log.Printf("Error downloading image. Status: %d", imageResp.StatusCode)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to download image",
			fmt.Sprintf("Image server returned status: %d", imageResp.StatusCode))
		return
	}

	imageBytes, err := io.ReadAll(imageResp.Body)
	if err != nil {
		log.Printf("Error reading image data: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to read image data", err.Error())
		return
	}

	log.Printf("Successfully read image data. Size: %d bytes", len(imageBytes))

	// Send identification request with retry
	responseBody, err := moService.IdentifyWithRetry(imageBytes, req.APIKey)
	if err != nil {
		log.Printf("Identification failed: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to identify image", err.Error())
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

	server := &http.Server{
		Addr:              ":" + port,
		ReadTimeout:       300 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 60 * time.Second,
	}

	http.HandleFunc("/identify", handlers.EnableCORS(identifyHandler))

	log.Printf("Server started on port %s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
