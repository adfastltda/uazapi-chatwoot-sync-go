package chatwoot

import (
	"bytes"
	"chatwoot-sync-go/internal/config"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type APIClient struct {
	baseURL   string
	token     string
	accountID int
	client    *http.Client
}

func NewAPIClient(cfg *config.Config) *APIClient {
	return &APIClient{
		baseURL:   cfg.Chatwoot.API.BaseURL,
		token:     cfg.Chatwoot.API.Token,
		accountID: cfg.Chatwoot.AccountID,
		client: &http.Client{
			Timeout: 120 * time.Second, // Timeout maior para uploads de mídia
		},
	}
}

// CreateMessageWithAttachment cria uma mensagem com attachment via API do Chatwoot
func (c *APIClient) CreateMessageWithAttachment(
	conversationID int,
	content string,
	messageType string, // "incoming" ou "outgoing"
	attachmentURL string,
	attachmentType string, // "image", "video", "audio", "file"
	sourceID string,
) (map[string]interface{}, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, fmt.Errorf("Chatwoot API not configured (CHATWOOT_BASE_URL and CHATWOOT_API_TOKEN required)")
	}

	// Determinar se é data URL (base64) ou file URL
	isDataURL := strings.HasPrefix(attachmentURL, "data:")
	
	var fileData []byte
	var fileName string
	var mimeType string

	if isDataURL {
		// Extrair base64 do data URL
		parts := strings.Split(attachmentURL, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URL format")
		}
		
		// Extrair mime type do data URL
		dataPart := parts[0]
		if strings.Contains(dataPart, ";base64") {
			mimeType = strings.TrimPrefix(strings.Split(dataPart, ";")[0], "data:")
		}
		
		var err error
		fileData, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64: %w", err)
		}
		
		// Gerar nome de arquivo baseado no tipo
		ext := c.getExtensionFromMimeType(mimeType)
		fileName = fmt.Sprintf("media_%d%s", time.Now().Unix(), ext)
	} else {
		// Baixar arquivo da URL
		resp, err := http.Get(attachmentURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download file: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to download file: status %d", resp.StatusCode)
		}

		fileData, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		// Extrair nome de arquivo e mime type da URL
		fileName = filepath.Base(attachmentURL)
		mimeType = resp.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = mime.TypeByExtension(filepath.Ext(fileName))
		}
	}

	// Criar FormData
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Adicionar campos
	if content != "" {
		writer.WriteField("content", content)
	}
	writer.WriteField("message_type", messageType)
	if sourceID != "" {
		writer.WriteField("source_id", sourceID)
	}

	// Adicionar arquivo
	part, err := writer.CreateFormFile("attachments[]", fileName)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(fileData); err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	writer.Close()

	// Criar requisição
	url := fmt.Sprintf("%s/api/v1/accounts/%d/conversations/%d/messages", 
		c.baseURL, c.accountID, conversationID)
	
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api_access_token", c.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Executar requisição
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Successfully created message with attachment via API for conversation %d (source_id: %s)", 
		conversationID, sourceID)
	return result, nil
}

func (c *APIClient) getExtensionFromMimeType(mimeType string) string {
	exts, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(exts) == 0 {
		// Fallback para tipos comuns
		switch {
		case strings.HasPrefix(mimeType, "image/"):
			if strings.Contains(mimeType, "webp") {
				return ".webp"
			}
			return ".jpg"
		case strings.HasPrefix(mimeType, "video/"):
			return ".mp4"
		case strings.HasPrefix(mimeType, "audio/"):
			return ".mp3"
		default:
			return ".bin"
		}
	}
	return exts[0]
}

