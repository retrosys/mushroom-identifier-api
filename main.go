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

	// Récupérer et logger le token d'authentification pour debug
	authHeader := r.Header.Get("Authorization")
	log.Printf("Received Authorization header: %s", authHeader)

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

	// Debug: Afficher l'URL de la requête
	log.Printf("Sending request to iNaturalist URL: %s", inatRequest.URL.String())

	inatRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	inatRequest.Header.Set("Accept", "application/json")
	inatRequest.Header.Set("User-Agent", "Mushroom Identifier/1.0")
	
	// S'assurer que le token est bien formaté avant de l'envoyer
	if authHeader != "" {
		log.Printf("Setting Authorization header for iNaturalist: %s", authHeader)
		inatRequest.Header.Set("Authorization", authHeader)
	} else {
		log.Printf("Warning: No Authorization header received from client")
	}

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
src/lib/mushroom-classifier.ts
interface iNatResult {
  score: number;
  taxon: {
    name: string;
    preferred_common_name: string;
    wikipedia_url?: string;
    default_photo?: {
      url?: string;
    };
  };
}

// Le token a été obtenu via l'interface web d'iNaturalist
const API_TOKEN = "eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjo4OTUxNjYwLCJleHAiOjE3NDEwMDk2MDB9.V9yC7M8QnLzM4u7LMpz7vkC2SzgUqcxRV1TnLpKo5vQ";

export const identifyMushroom = async (imageUrl: string) => {
  try {
    console.log("Début de l'identification avec l'URL:", imageUrl);
    console.log("Token utilisé:", `JWT ${API_TOKEN}`); // Debug: afficher le token
    
    // Appel à notre backend sur Render
    const response = await fetch('https://mushroom-identifier-api.onrender.com/identify', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `JWT ${API_TOKEN}`,
      },
      body: JSON.stringify({ imageUrl })
    });

    console.log("Statut de la réponse:", response.status); // Debug: afficher le statut

    if (!response.ok) {
      const errorText = await response.text();
      console.error("Erreur détaillée:", errorText); // Debug: afficher l'erreur complète
      
      if (response.status === 401) {
        throw new Error("Erreur d'authentification avec le service d'identification. Nous travaillons à résoudre ce problème.");
      }
      if (response.status === 403) {
        throw new Error("Accès refusé par le service d'identification. Nous travaillons à résoudre ce problème.");
      }
      throw new Error(`Erreur du service d'identification: ${response.status} ${response.statusText}`);
    }

    const data = await response.json();
    console.log("Résultats bruts iNaturalist:", data);

    if (!data || !data.results) {
      throw new Error("Le service d'identification n'a pas retourné de résultats valides.");
    }

    // Filtrer uniquement les champignons (kingdom Fungi)
    const mushroomResults = data.results.filter((result: iNatResult) => 
      result.taxon.name.toLowerCase().includes('fungi') || 
      result.taxon.name.toLowerCase().includes('mushroom')
    );

    if (mushroomResults.length === 0) {
      throw new Error("Aucun champignon n'a été identifié dans cette image.");
    }

    // Formater les résultats
    return mushroomResults.map((result: iNatResult) => ({
      label: result.taxon.preferred_common_name || result.taxon.name,
      confidence: result.score,
      wikiUrl: result.taxon.wikipedia_url,
      imageUrl: result.taxon.default_photo?.url
    }));

  } catch (error) {
    console.error("Erreur détaillée lors de l'identification:", error);
    
    if (error instanceof Error) {
      // Si l'erreur a déjà un message personnalisé, on le renvoie directement
      if (error.message.includes("service d'identification") || 
          error.message.includes("champignon") ||
          error.message.includes("résultats valides")) {
        throw error;
      }
    }
    
    // Message d'erreur par défaut
    throw new Error("Une erreur est survenue lors de l'identification. Veuillez réessayer plus tard.");
  }
};
