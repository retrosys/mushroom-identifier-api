package services

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/retrosys/mushroom-identifier-api/utils"
)

const (
	moBaseURL    = "https://mushroomobserver.org/api2"
	maxRetries   = 3
	retryTimeout = 5 * time.Second
)

func SendIdentificationRequest(imageData []byte, apiKey string) ([]byte, error) {
	client := utils.NewHTTPClient(60 * time.Second)

	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	if err := multipartWriter.WriteField("api_key", apiKey); err != nil {
		return nil, fmt.Errorf("error writing api_key field: %v", err)
	}

	if err := multipartWriter.WriteField("method", "identify_image"); err != nil {
		return nil, fmt.Errorf("error writing method field: %v", err)
	}

	filePart, err := multipartWriter.CreateFormFile("file", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("error creating form file: %v", err)
	}

	if _, err := filePart.Write(imageData); err != nil {
		return nil, fmt.Errorf("error writing image data: %v", err)
	}

	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("error closing multipart writer: %v", err)
	}

	request, err := http.NewRequest("POST", moBaseURL, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Mushroom Identifier/1.0")

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", response.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
