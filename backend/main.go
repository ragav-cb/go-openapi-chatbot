package main

import (
	"bytes"
	"chatbot-backend/agent"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"

	"github.com/joho/godotenv"
)

/* type ChatRequest struct {
	Message string `json:"message"`
} */

type OpenAIRequest struct {
	Model    string              `json:"model"`
	Messages []map[string]string `json:"messages"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

/* type ChatResponse struct {
	Reply string `json:"reply"`
} */

func main() {
	_ = godotenv.Load()

	http.HandleFunc("/api/chat", chatHandler)
	http.HandleFunc("/assistant", withCORS(agent.CreateAssistantHandler))
	http.HandleFunc("/upload", withCORS(agent.UploadFileHandler))
	http.HandleFunc("/assistant/add-file", agent.AddFileToAssistantHandler)
	http.HandleFunc("/assistant/query", withCORS(agent.QueryAssistantHandler))

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Allow any frontend origin during development
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		h(w, r) // call the original handler
	}
}

func searchConfluence(query string) (string, error) {
	base := "https://cloudbees.atlassian.net/wiki/rest/api/content/search"
	searchURL := fmt.Sprintf("%s?cql=text~\"%s\"&expand=body.storage", base, query)

	req, _ := http.NewRequest("GET", searchURL, nil)
	req.SetBasicAuth("your-email@example.com", "your-api-token")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	var result struct {
		Results []struct {
			Title string `json:"title"`
			Body  struct {
				Storage struct {
					Value string `json:"value"`
				} `json:"storage"`
			} `json:"body"`
		} `json:"results"`
	}

	json.Unmarshal(body, &result)

	if len(result.Results) == 0 {
		return "", fmt.Errorf("No relevant docs found.")
	}

	// Return plain-text version of first hit
	return result.Results[0].Title + "\n\n" + stripHTML(result.Results[0].Body.Storage.Value), nil
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
func stripHTML(input string) string {
	// Remove tags like <p>, <br>, <a>, etc.
	re := regexp.MustCompile("<[^>]*>")
	return re.ReplaceAllString(input, "")
}
func chatHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Incoming request:", r.Method, r.Body, r.URL)
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading request body:", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.Println("Raw body received:", string(body))

	// Restore r.Body so it can be used again
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var userInput ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&userInput); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	log.Println("Input request:", userInput.Messages)

	log.Printf("Parsed request: Model=%s, Messages=%+v\n", userInput.Model, userInput.Messages)

	var prompt string

	// Try fetching docs from Confluence
	if len(userInput.Messages) > 0 {
		log.Println("Process search confluence:", userInput.Messages[0].Content)
		docs, err := searchConfluence(userInput.Messages[0].Content)
		if err == nil && docs != "" {
			prompt = fmt.Sprintf("Use the following documentation to help answer the question:\n\n%s\n\nQuestion: %s", docs, userInput.Messages[0].Content)
		} else {
			// Fallback to plain OpenAI query
			log.Println("Error in confluence search. :", err)
			log.Println("Fallback to plain OpenAI query. :", userInput.Messages[0].Content)
			prompt = userInput.Messages[0].Content
		}
	} else {
		// Fallback to plain OpenAI query
		prompt = "No Input provided. Please ask a question."
	}

	// Send to OpenAI
	reply, err := queryOpenAI(prompt)
	if err != nil {
		log.Printf("OpenAI API error: %v", err)
		http.Error(w, "OpenAI error", http.StatusInternalServerError)
		return
	}
	log.Println("Response sent:", reply)
	//json.NewEncoder(w).Encode(ChatResponse{Reply: reply})
	json.NewEncoder(w).Encode(ChatResponse{Choices: []ChatChoice{{Message: ChatMessage{Content: reply}}}})
}

/* func processWithOpenAI(r *http.Request, userInput ChatRequest, w http.ResponseWriter) bool {
	if err := json.NewDecoder(r.Body).Decode(&userInput); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return true
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		http.Error(w, "API key not set", http.StatusInternalServerError)
		return true
	}

	payload := OpenAIRequest{
		//Model: "gpt-3.5-turbo",
		Model: "gpt-4.1-mini",
		Messages: []map[string]string{
			{"role": "user", "content": userInput.Message},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "OpenAI API error", http.StatusInternalServerError)
		return true
	}
	defer resp.Body.Close()

	data, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		http.Error(w, string(data), resp.StatusCode)
		return true
	}

	var aiResp OpenAIResponse
	json.Unmarshal(data, &aiResp)

	if len(aiResp.Choices) == 0 {
		http.Error(w, "No response from OpenAI", http.StatusInternalServerError)
		return true
	}

	json.NewEncoder(w).Encode(ChatResponse{Reply: aiResp.Choices[0].Message.Content})
	return false
}
*/
