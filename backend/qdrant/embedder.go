package qdrant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	url := "https://api.openai.com/v1/embeddings"
	input := []string{"Confluence content to embed"}

	reqBody := EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: input,
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		log.Fatal(err)
	}

	fmt.Println(embeddingResp.Data[0].Embedding)
}
