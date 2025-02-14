package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
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

	// Télécharger l'image
	resp, err := http.Get(req.ImageURL)
	if err != nil {
		http.Error(w, "Erreur lors du téléchargement de l'image", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Lire l'image
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Erreur lors de la lecture de l'image", http.StatusInternalServerError)
		return
	}

	// Préparer la requête pour iNaturalist
	body := &bytes.Buffer{}
	writer := http.NewWriter(body)
	part, err := writer.CreateFormFile("image", "image.jpg")
	if err != nil {
		http.Error(w, "Erreur lors de la création de la requête", http.StatusInternalServerError)
		return
	}
	part.Write(imageData)
	writer.Close()

	// Appeler l'API iNaturalist
	inatReq, err := http.NewRequest("POST", "https://api.inaturalist.org/v1/computervision/score_image", body)
	if err != nil {
		http.Error(w, "Erreur lors de la création de la requête iNaturalist", http.StatusInternalServerError)
		return
	}
	inatReq.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	inatResp, err := client.Do(inatReq)
	if err != nil {
		http.Error(w, "Erreur lors de l'appel à iNaturalist", http.StatusInternalServerError)
		return
	}
	defer inatResp.Body.Close()

	// Copier la réponse d'iNaturalist
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(inatResp.StatusCode)
	io.Copy(w, inatResp.Body)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/identify", enableCORS(identifyHandler))

	log.Printf("Serveur démarré sur le port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
