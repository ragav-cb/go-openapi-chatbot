package agent

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

func CreateAssistantHandler(w http.ResponseWriter, r *http.Request) {

	var reqBody CreateAssistantRequest

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	log.Println("Incoming agent request:", reqBody.Name, reqBody.Instructions, reqBody.Model)

	assistantID, err := CreateAssistant(reqBody.Name, reqBody.Instructions, reqBody.Model)
	if err != nil {
		http.Error(w, "Failed to create assistant", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Assistant created with ID: %s", assistantID)
}

// CreateAssistant defines your AI agent
func CreateAssistant(name, instructions string, model string) (string, error) {
	payload := map[string]interface{}{
		"name":         name,
		"instructions": instructions,
		"model":        model,
		"tools":        []map[string]string{{"type": "file_search"}},
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

	err = UpdateAssistantWithFiles("asst_dsaGJnHziOJd94Kr37uJv3r9", []string{fileID}, []string{vectorStoreID})
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
func QueryAssistant(assistantID, message string) error {
	// 1. Create Thread
	threadPayload := map[string]interface{}{}
	threadJSON, _ := json.Marshal(threadPayload)

	req, _ := http.NewRequest("POST", BASE_URL+"/threads", bytes.NewBuffer(threadJSON))
	req.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var threadRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&threadRes)
	threadID := threadRes["id"].(string)

	// 2. Add message to thread
	msgPayload := map[string]interface{}{
		"role":    "user",
		"content": message,
	}
	msgJSON, _ := json.Marshal(msgPayload)
	msgReq, _ := http.NewRequest("POST", BASE_URL+"/threads/"+threadID+"/messages", bytes.NewBuffer(msgJSON))
	msgReq.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	msgReq.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(msgReq)

	// 3. Run the assistant
	runPayload := map[string]interface{}{
		"assistant_id": assistantID,
	}
	runJSON, _ := json.Marshal(runPayload)
	runReq, _ := http.NewRequest("POST", BASE_URL+"/threads/"+threadID+"/runs", bytes.NewBuffer(runJSON))
	runReq.Header.Set("Authorization", "Bearer "+OPENAI_API_KEY)
	runReq.Header.Set("Content-Type", "application/json")

	runResp, _ := http.DefaultClient.Do(runReq)
	defer runResp.Body.Close()

	body, _ := io.ReadAll(runResp.Body)
	fmt.Println("Assistant Response:", string(body))
	return nil
}
func QueryAssistantHandler(w http.ResponseWriter, r *http.Request) {
	// Get the assistant ID and message from the request
	assistantID := r.URL.Query().Get("assistant_id")
	message := r.URL.Query().Get("message")

	// Call the QueryAssistant function
	err := QueryAssistant(assistantID, message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return a success response
	w.WriteHeader(http.StatusOK)
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

/*

func main() {
	// 1. Upload a file (optional, used if you want retrieval)
	fileID, _ := UploadFile("example.pdf")

	// 2. Create assistant with file
	assistantID, _ := CreateAssistant("Demo Assistant", "You are a helpful AI that uses uploaded documents.", "gpt-4-1106-preview")

	// 3. Query it
	QueryAssistant(assistantID, "What is this document about?")
} */
