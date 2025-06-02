package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	oauth2Config = oauth2.Config{
		ClientID:     os.Getenv("CONFLUENCE_CLIENT_ID"),
		ClientSecret: os.Getenv("CONFLUENCE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("CONFLUENCE_REDIRECT_URI"),
		Scopes:       []string{"read:confluence-content"},
		Endpoint:     google.Endpoint,
	}
	stateToken = "random_state_token"
	//oauth2StateString = "random"
	/* conf              = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{"read:confluence-content.all", "offline_access"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://auth.atlassian.com/authorize",
			TokenURL: "https://auth.atlassian.com/oauth/token",
		},
	} */
)

/*
func main() {
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/callback", callbackHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
} */

func main() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/oauth/callback", handleOAuthCallback)
	log.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	url := oauth2Config.AuthCodeURL(stateToken)
	http.Redirect(w, r, url, http.StatusFound)
}

func getCloudID(accessToken string) string {
	client := resty.New()
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+accessToken).
		Get("https://api.atlassian.com/oauth/token/accessible-resources")

	if err != nil || resp.IsError() {
		log.Println("Failed to get accessible resources:", err)
		return ""
	}

	var resources []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	json.Unmarshal(resp.Body(), &resources)
	if len(resources) == 0 {
		return ""
	}
	return resources[0].ID // first site
}
func getConfluencePage(cloudID, pageID, accessToken string) string {
	client := resty.New()
	url := fmt.Sprintf("https://api.atlassian.com/ex/confluence/%s/wiki/rest/api/content/%s?expand=body.storage", cloudID, pageID)
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil || resp.IsError() {
		log.Println("Error fetching page:", err)
		return ""
	}

	var result struct {
		Title string `json:"title"`
		Body  struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}
	json.Unmarshal(resp.Body(), &result)
	return fmt.Sprintf("Title: %s\n\nContent:\n%s", result.Title, result.Body.Storage.Value)
}
func handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("state") != stateToken {
		http.Error(w, "State mismatch", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")

	tok, err := oauth2Config.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Token exchange error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get cloud ID (required for API requests)
	cloudID := getCloudID(tok.AccessToken)
	if cloudID == "" {
		http.Error(w, "Failed to get cloud ID", http.StatusInternalServerError)
		return
	}

	// Call Confluence API
	content := getConfluencePage(cloudID, "your-page-id", tok.AccessToken)
	fmt.Fprintf(w, "Fetched content:\n\n%s", content)
}
func loginHandler(w http.ResponseWriter, r *http.Request) {
	url := oauth2Config.AuthCodeURL(oauth2StateString, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	token, err := oauth2Config.Exchange(oauth2.NoContext, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to exchange token: %v", err), http.StatusInternalServerError)
		return
	}
	// Use token to make API requests
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.atlassian.com/ex/confluence/1/rest/api/content", nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to make API request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "API response: %s", resp.Status)
}
