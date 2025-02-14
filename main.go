package main

import (
    "bytes"
    "encoding/json"
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

    // Télécharger l'image depuis l'URL
    imageResp, err := http.Get(req.ImageURL)
    if err != nil {
        http.Error(w, "Failed to download image", http.StatusInternalServerError)
        return
    }
    defer imageResp.Body.Close()

    // Lire l'image en mémoire
    imageBytes, err := io.ReadAll(imageResp.Body)
    if err != nil {
        http.Error(w, "Failed to read image", http.StatusInternalServerError)
        return
    }

    // Créer un buffer pour stocker la requête multipart
    var requestBody bytes.Buffer
    multipartWriter := multipart.NewWriter(&requestBody)

    // Créer la partie fichier dans la requête multipart
    filePart, err := multipartWriter.CreateFormFile("image", "image.jpg")
    if err != nil {
        http.Error(w, "Failed to create form file", http.StatusInternalServerError)
        return
    }

    // Écrire les données de l'image dans la partie fichier
    if _, err := filePart.Write(imageBytes); err != nil {
        http.Error(w, "Failed to write image data", http.StatusInternalServerError)
        return
    }

    // Fermer le writer multipart
    if err := multipartWriter.Close(); err != nil {
        http.Error(w, "Failed to close multipart writer", http.StatusInternalServerError)
        return
    }

    // Créer la requête vers iNaturalist
    inatRequest, err := http.NewRequest("POST", "https://api.inaturalist.org/v1/computervision/score_image", &requestBody)
    if err != nil {
        http.Error(w, "Failed to create request", http.StatusInternalServerError)
        return
    }

    // Définir le Content-Type pour la requête multipart
    inatRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())

    // Envoyer la requête
    client := &http.Client{}
    inatResponse, err := client.Do(inatRequest)
    if err != nil {
        http.Error(w, "Failed to send request", http.StatusInternalServerError)
        return
    }
    defer inatResponse.Body.Close()

    // Copier la réponse vers le client
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(inatResponse.StatusCode)
    if _, err := io.Copy(w, inatResponse.Body); err != nil {
        log.Printf("Error copying response: %v", err)
    }
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
