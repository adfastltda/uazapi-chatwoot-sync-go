package uazapi

import (
	"bytes"
	"chatwoot-sync-go/internal/config"
	"chatwoot-sync-go/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL: cfg.UAZAPI.BaseURL,
		token:   cfg.UAZAPI.Token,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// FindChats busca chats da API UAZAPI
func (c *Client) FindChats(limit, offset int, isGroup bool) (*models.UAZAPIChatsResponse, error) {
	url := fmt.Sprintf("%s/chat/find", c.baseURL)

	payload := map[string]interface{}{
		"operator":     "LIKE",
		"sort":         "-wa_lastMsgTimestamp",
		"limit":        limit,
		"offset":       offset,
		"wa_isGroup":   isGroup,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result models.UAZAPIChatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Found %d chats (offset: %d)", len(result.Chats), offset)
	return &result, nil
}

// FindMessages busca mensagens de um chat específico
func (c *Client) FindMessages(chatID string, limit, offset int) (*models.UAZAPIMessagesResponse, error) {
	url := fmt.Sprintf("%s/message/find", c.baseURL)

	payload := map[string]interface{}{
		"chatid": chatID,
		"limit":  limit,
		"offset": offset,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result models.UAZAPIMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Found %d messages for chat %s (offset: %d)", len(result.Messages), chatID, offset)
	return &result, nil
}

// GetAllChats busca todos os chats (com paginação)
func (c *Client) GetAllChats(limit int, isGroup bool) ([]models.UAZAPIChat, error) {
	var allChats []models.UAZAPIChat
	offset := 0

	for {
		response, err := c.FindChats(limit, offset, isGroup)
		if err != nil {
			return nil, err
		}

		allChats = append(allChats, response.Chats...)

		if !response.Pagination.HasNextPage {
			break
		}

		offset += len(response.Chats)
		log.Printf("Fetched %d chats so far...", len(allChats))
	}

	return allChats, nil
}

// GetAllMessages busca todas as mensagens de um chat (com paginação)
func (c *Client) GetAllMessages(chatID string, limit int) ([]models.UAZAPIMessage, error) {
	var allMessages []models.UAZAPIMessage
	offset := 0

	for {
		response, err := c.FindMessages(chatID, limit, offset)
		if err != nil {
			return nil, err
		}

		allMessages = append(allMessages, response.Messages...)

		if !response.HasMore {
			break
		}

		offset = response.NextOffset
		log.Printf("Fetched %d messages for chat %s so far...", len(allMessages), chatID)
	}

	return allMessages, nil
}

// DownloadMedia baixa uma mídia usando o messageid
func (c *Client) DownloadMedia(messageID string) (*models.UAZAPIMediaResponse, error) {
	url := fmt.Sprintf("%s/message/download", c.baseURL)

	payload := map[string]interface{}{
		"id":             messageID,
		"return_base64":  true,
		"return_link":    true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result models.UAZAPIMediaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

