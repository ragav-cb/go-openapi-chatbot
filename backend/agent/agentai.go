package agent

import (
	"bytes"
	"chatbot-backend/models"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

var OPENAI_API_KEY string

func init() {
	OPENAI_API_KEY = os.Getenv("OPENAI_API_KEY")
}

const BASE_URL = "https://api.openai.com/v1"

type CreateAssistantRequest struct {
	Name         string `json:"name"`
	Instructions string `json:"instructions"`
	Model        string `json:"model"`
}
type ChatMessage = models.ChatMessage
type ChatRequest = models.ChatRequest
type ChatResponse = models.ChatResponse

func CreateAssistantHandler(w http.ResponseWriter, r *http.Request) {

	var reqBody CreateAssistantRequest

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	log.Println("Incoming agent request:", reqBody.Name, reqBody.Instructions, reqBody.Model)
	/* backend-1   | Uploaded File ID: file-H7MSZVo3s8cgkKiNY2nViL
	backend-1   | 2025/06/02 19:32:25 After json unmarshalling openai upload request file-H7MSZVo3s8cgkKiNY2nViL
	backend-1   | Vector Store ID: vs_683dfc4aeafc81918a9a6da095bda478
	*/
	assistantID, err := CreateAssistant(reqBody.Name, reqBody.Instructions, reqBody.Model, "vs_683dfc4aeafc81918a9a6da095bda478", []string{"file-H7MSZVo3s8cgkKiNY2nViL"})
	if err != nil {
		http.Error(w, "Failed to create assistant", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ChatResponse{Choices: []models.ChatChoice{{Message: ChatMessage{Content: assistantID}}}})
	fmt.Fprintf(w, "Assistant created with ID: %s", assistantID)
}

// CreateAssistant defines your AI agent
func CreateAssistant(name, instructions string, model string, vectorStoreID string, fileIDs []string) (string, error) {
	payload := map[string]interface{}{
		"name":         name,
		"instructions": instructions,
		"model":        model,
		"tools": []map[string]interface{}{
			{"type": "code_interpreter"},
			{"type": "file_search"},
		},
		"tool_resources": map[string]interface{}{
			"file_search": map[string]interface{}{
				"vector_store_ids": []string{vectorStoreID},
			},
			"code_interpreter": map[string]interface{}{
				"file_ids": fileIDs,
			},
		},
	}

	jsonData, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", BASE_URL+"/assistants", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Printf("OpenAI API error (%d): %s\n", resp.StatusCode, body)
		return "", fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	idRaw, ok := result["id"]
	if !ok {
		return "", fmt.Errorf("missing 'id' in response: %s", string(body))
	}
	assistantID := idRaw.(string)
	fmt.Println("Created Assistant ID:", assistantID)
	return assistantID, nil
}

func CreateVectorStore(name string, fileIDs []string) (string, error) {
	payload := map[string]interface{}{
		"name":     name,
		"file_ids": fileIDs,
		"metadata": map[string]string{"purpose": "retrieval"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal vector store payload: %w", err)
	}

	req, err := http.NewRequest("POST", BASE_URL+"/vector_stores", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create vector store request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send vector store request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("OpenAI vector store creation failed (%d): %s", resp.StatusCode, respBody)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	vectorStoreID, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("missing vector store id in response: %s", respBody)
	}

	return vectorStoreID, nil
}

// Add file to assistant using the assistant ID and file ID
func AddFileToAssistant(assistantID, fileID string) error {
	log.Println("Adding file to assistant:", assistantID, fileID)
	payload := map[string]interface{}{
		"file_id": fileID,
	}
	jsonData, _ := json.Marshal(payload)
	log.Println("after successful json marshal")
	url := fmt.Sprintf("%s/assistants/%s/files", BASE_URL, assistantID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	log.Println("After making openai request")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Error adding file to assistant:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to attach file: %s", string(body))
	}

	fmt.Println("File attached to assistant.")
	return nil
}

func UploadFileHandler(w http.ResponseWriter, r *http.Request) {

	log.Println("Incoming file upload request")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20) // Max upload size: 10MB
	if err != nil {
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		log.Println("Error uploading file:", err)
		http.Error(w, "Error uploading file", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	// Save to temp file (optional, or just send directly to OpenAI)
	tempFile, err := os.CreateTemp("", fileHeader.Filename)
	if err != nil {
		http.Error(w, "Failed to create temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name()) // Optional cleanup
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Now send tempFile to OpenAI
	openAIFile, err := os.Open(tempFile.Name())
	if err != nil {
		http.Error(w, "Failed to re-open temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer openAIFile.Close()

	fileID, err := UploadFile(openAIFile, fileHeader.Filename)
	if err != nil {
		http.Error(w, "Error uploading file to OpenAI", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File uploaded. ID: %s", fileID)
}

// UploadFile uploads a file to OpenAI's file endpoint
func UploadFile(file io.Reader, filePath string) (string, error) {
	log.Println("Uploading file:", filePath)

	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	part, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		log.Println("Error creating form file:", err)
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	writer.WriteField("purpose", "assistants")
	writer.Close()
	log.Println("before making opanai upload request")
	req, _ := http.NewRequest("POST", BASE_URL+"/files", &b)
	req.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	log.Println("After making openai upload request")
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	idRaw, ok := result["id"]
	if !ok {
		return "", fmt.Errorf("missing 'id' in response: %s", string(body))
	}
	fileID, ok := idRaw.(string)
	if !ok {
		return "", fmt.Errorf("'id' is not a string: %v", idRaw)
	}
	fmt.Println("Uploaded File ID:", fileID)
	log.Println("After json unmarshalling openai upload request", fileID)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Printf("OpenAI API error (%d): %s\n", resp.StatusCode, body)
		return "", fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}
	if fileID == "" {
		return "", fmt.Errorf("missing 'id' in response: %s", string(body))
	}

	vectorStoreID, err := CreateVectorStore("My Docs Store", []string{fileID})
	if err != nil {
		log.Fatal("Vector store creation failed:", err)
	}
	fmt.Println("Vector Store ID:", vectorStoreID)

	err = UpdateAssistantWithFiles("asst_lt0oTIA4NPIRmzsdssSdqZNl", []string{fileID}, []string{vectorStoreID})
	if err != nil {
		//http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error adding file to assistant:", err)
		return "", err
	}
	log.Println("File attached to assistant.")
	return fileID, nil
}

func UpdateAssistantWithFiles(assistantID string, fileIDs []string, vectorStoreIDs []string) error {
	updatePayload := map[string]interface{}{
		"tools": []map[string]interface{}{
			{"type": "code_interpreter"},
			{"type": "file_search"},
		},
		"tool_resources": map[string]interface{}{
			"code_interpreter": map[string]interface{}{
				"file_ids": fileIDs,
			},
			"file_search": map[string]interface{}{
				"vector_store_ids": vectorStoreIDs,
			},
		},
	}

	body, err := json.Marshal(updatePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal update payload: %w", err)
	}

	url := fmt.Sprintf("%s/assistants/%s", BASE_URL, assistantID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create update request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Error adding file to assistant:", err)
		return fmt.Errorf("failed to send update request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenAI update failed (%d): %s", resp.StatusCode, respBody)
	}

	log.Println("Assistant updated successfully.")
	return nil
}

// QueryAssistant creates a thread, adds a message, and runs it
func QueryAssistant(assistantID, message string) (string, error) {
	// 1. Create Thread
	threadPayload := map[string]interface{}{}
	threadJSON, _ := json.Marshal(threadPayload)
	log.Println("after successful json marshal API Key", OPENAI_API_KEY)
	req, _ := http.NewRequest("POST", BASE_URL+"/threads", bytes.NewBuffer(threadJSON))
	req.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var threadRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&threadRes)
	threadID := threadRes["id"].(string)
	log.Println("Content sent to agent", message)
	// 2. Add message to thread
	msgPayload := map[string]interface{}{
		"role":    "user",
		"content": message,
	}
	msgJSON, _ := json.Marshal(msgPayload)
	msgReq, _ := http.NewRequest("POST", BASE_URL+"/threads/"+threadID+"/messages", bytes.NewBuffer(msgJSON))
	msgReq.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	msgReq.Header.Set("Content-Type", "application/json")
	msgReq.Header.Set("OpenAI-Beta", "assistants=v2")
	http.DefaultClient.Do(msgReq)

	// 3. Run the assistant
	runPayload := map[string]interface{}{
		"assistant_id": assistantID,
	}
	runJSON, _ := json.Marshal(runPayload)
	runReq, _ := http.NewRequest("POST", BASE_URL+"/threads/"+threadID+"/runs", bytes.NewBuffer(runJSON))
	runReq.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	runReq.Header.Set("Content-Type", "application/json")
	runReq.Header.Set("OpenAI-Beta", "assistants=v2")

	runResp, _ := http.DefaultClient.Do(runReq)
	defer runResp.Body.Close()

	/* body, _ := io.ReadAll(runResp.Body)
	fmt.Println("Assistant Response:", string(body))
	return nil */

	var runRes map[string]interface{}
	json.NewDecoder(runResp.Body).Decode(&runRes)
	runID := runRes["id"].(string)

	// 4. Poll for run completion
	for {
		time.Sleep(2 * time.Second)
		statusReq, _ := http.NewRequest("GET", BASE_URL+"/threads/"+threadID+"/runs/"+runID, nil)
		statusReq.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
		statusReq.Header.Set("OpenAI-Beta", "assistants=v2")

		statusResp, err := http.DefaultClient.Do(statusReq)
		if err != nil {
			return "", err
		}
		defer statusResp.Body.Close()

		body, err := io.ReadAll(statusResp.Body)
		if err != nil {
			return "", err
		}

		var status map[string]interface{}
		if err := json.Unmarshal(body, &status); err != nil {
			log.Println("Failed to parse status response:", string(body))
			return "", err
		}

		statusStr, _ := status["status"].(string)

		if statusStr == "completed" {
			break
		} else if statusStr == "failed" {
			log.Println("Run failed response:", string(body))
			return "", fmt.Errorf("assistant run failed")
		}
	}

	// 5. Get messages
	msgListReq, _ := http.NewRequest("GET", BASE_URL+"/threads/"+threadID+"/messages", nil)
	msgListReq.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	msgListReq.Header.Set("OpenAI-Beta", "assistants=v2")
	msgListResp, err := http.DefaultClient.Do(msgListReq)
	if err != nil {
		return "", err
	}
	defer msgListResp.Body.Close()

	var msgList map[string]interface{}
	json.NewDecoder(msgListResp.Body).Decode(&msgList)

	// 6. Extract assistant's message
	if data, ok := msgList["data"].([]interface{}); ok {
		for _, item := range data {
			msg := item.(map[string]interface{})
			if msg["role"] == "assistant" {
				content := msg["content"].([]interface{})[0].(map[string]interface{})
				if text, ok := content["text"].(map[string]interface{}); ok {
					return text["value"].(string), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no assistant message found")
}
func QueryAssistantHandler(w http.ResponseWriter, r *http.Request) {
	// Get the assistant ID and message from the request
	assistantID := r.URL.Query().Get("assistant_id")
	var userInput ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&userInput); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	log.Println("Input request:", userInput.Messages)

	log.Printf("Parsed request: Model=%s, Messages=%+v\n", userInput.Model, userInput.Messages)

	log.Println(" plain OpenAI Agent query. :", userInput.Messages[0].Content)
	assistantID = "asst_lt0oTIA4NPIRmzsdssSdqZNl" // Example hardcoded ID, replace with actual ID from request
	// Call the QueryAssistant function
	response, err := QueryAssistant(assistantID, userInput.Messages[0].Content)
	if err != nil {
		log.Println("Error querying assistant:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return a success response
	w.Header().Set("Content-Type", "application/json")
	/* json.NewEncoder(w).Encode(map[string]string{
		"response": response,
	}) */
	json.NewEncoder(w).Encode(ChatResponse{Choices: []models.ChatChoice{{Message: ChatMessage{Content: response}}}})
}
func AddFileToAssistantHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		AssistantID string `json:"assistant_id"`
		FileID      string `json:"file_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	err := AddFileToAssistant(reqBody.AssistantID, reqBody.FileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func GetAssistantDetails(assistantID string) ([]byte, error) {
	req, _ := http.NewRequest("GET", BASE_URL+"/assistants/"+assistantID, nil)
	req.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Error getting assistant:", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("OpenAI API returned status %d: %s\n", resp.StatusCode, string(body))
		return nil, fmt.Errorf("OpenAI API error: %s", string(body))
	}

	return body, nil
}

func GetAssistantDetailsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract assistant ID from query param: /get-assistant?id=asst_abc123
	assistantID := r.URL.Query().Get("id")
	if assistantID == "" {
		http.Error(w, "missing assistant ID", http.StatusBadRequest)
		return
	}

	data, err := GetAssistantDetails(assistantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
