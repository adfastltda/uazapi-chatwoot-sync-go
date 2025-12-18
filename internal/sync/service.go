package sync

import (
	"chatwoot-sync-go/internal/chatwoot"
	"chatwoot-sync-go/internal/config"
	"chatwoot-sync-go/internal/models"
	"chatwoot-sync-go/internal/uazapi"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
)

type Service struct {
	cfg         *config.Config
	uazapi      *uazapi.Client
	chatwoot    *chatwoot.Database
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		cfg:      cfg,
		uazapi:   uazapi.NewClient(cfg),
		stopChan: make(chan struct{}),
	}
}

func (s *Service) Start() error {
	// Conectar ao banco do Chatwoot
	db, err := chatwoot.NewDatabase(s.cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	s.chatwoot = db
	defer s.chatwoot.Close()

	// Criar cliente da API do Chatwoot
	// Obter inbox
	inboxID, err := s.chatwoot.GetInbox()
	if err != nil {
		return fmt.Errorf("failed to get inbox: %w", err)
	}
	log.Printf("Using inbox ID: %d", inboxID)

	// Obter usuário do Chatwoot
	chatwootUser, err := s.chatwoot.GetChatwootUser(s.cfg.Chatwoot.API.Token)
	if err != nil {
		return fmt.Errorf("failed to get chatwoot user: %w", err)
	}
	log.Printf("Chatwoot User: %s (ID: %d)", chatwootUser.UserType, chatwootUser.UserID)

	// Buscar todos os chats (apenas não-grupos)
	log.Println("Fetching chats from UAZAPI...")
	chats, err := s.uazapi.GetAllChats(s.cfg.Sync.LimitChats, false)
	if err != nil {
		return fmt.Errorf("failed to fetch chats: %w", err)
	}
	log.Printf("Found %d chats to sync", len(chats))

	// Processar chats em lotes
	batchSize := s.cfg.Sync.BatchSize
	for i := 0; i < len(chats); i += batchSize {
		select {
		case <-s.stopChan:
			log.Println("Sync stopped by user")
			return nil
		default:
		}

		end := i + batchSize
		if end > len(chats) {
			end = len(chats)
		}

		batch := chats[i:end]
		log.Printf("Processing batch %d-%d of %d chats", i+1, end, len(chats))

		if err := s.processChatsBatch(batch, inboxID, chatwootUser); err != nil {
			log.Printf("Error processing batch: %v", err)
			// Continue com próximo batch mesmo se houver erro
		}
	}

	log.Println("Sync completed successfully")
	return nil
}

func (s *Service) processChatsBatch(
	chats []models.UAZAPIChat,
	inboxID int,
	chatwootUser *models.ChatwootUser,
) error {
	// Preparar contatos apenas para chats que têm mensagens
	contacts := make([]models.ChatwootContact, 0, len(chats))
	chatMap := make(map[string]models.UAZAPIChat)
	chatsWithMessages := make(map[string]bool)

	// Primeiro, verificar quais chats têm mensagens
	log.Printf("Checking which chats have messages...")
	for _, chat := range chats {
		if chat.WAIsGroup || chat.Phone == "" {
			continue // Pular grupos e chats sem telefone
		}

		chatID := chat.WAChatID
		if chatID == "" {
			chatID = chat.WAChatLID
		}
		if chatID == "" {
			continue
		}

		// Verificar se o chat tem mensagens
		messages, err := s.uazapi.GetAllMessages(chatID, s.cfg.Sync.LimitMessages)
		if err != nil {
			log.Printf("Warning: failed to check messages for chat %s: %v", chatID, err)
			continue
		}

		if len(messages) == 0 {
			log.Printf("Skipping chat %s (phone: %s) - no messages", chatID, chat.Phone)
			continue // Ignorar chats sem mensagens
		}

		phoneNumber := s.normalizePhoneNumber(chat.Phone)
		if phoneNumber == "" {
			continue
		}

		// Chat tem mensagens, adicionar à lista
		chatsWithMessages[phoneNumber] = true
		contacts = append(contacts, models.ChatwootContact{
			PhoneNumber:   phoneNumber,
			Name:          s.getContactName(chat),
			Identifier:   s.buildIdentifier(phoneNumber),
			FirstTimestamp: chat.WALastMsgTimestamp,
			LastTimestamp:  chat.WALastMsgTimestamp,
		})

		chatMap[phoneNumber] = chat
	}

	if len(contacts) == 0 {
		log.Printf("No chats with messages to process")
		return nil
	}

	log.Printf("Found %d chats with messages out of %d total chats", len(contacts), len(chats))

	// Criar contatos e conversas apenas para chats com mensagens
	log.Printf("Creating/updating %d contacts and conversations...", len(contacts))
	fksMap, err := s.chatwoot.CreateContactsAndConversations(contacts, inboxID)
	if err != nil {
		return fmt.Errorf("failed to create contacts: %w", err)
	}

	log.Printf("Created/updated %d contacts and conversations", len(fksMap))
	
	if len(fksMap) == 0 {
		log.Printf("WARNING: No FKs returned from CreateContactsAndConversations, but %d contacts were provided", len(contacts))
		log.Printf("This might indicate that contacts already exist but query didn't return them, or query failed")
		return fmt.Errorf("no contacts/conversations created or found")
	}

	// Processar mensagens para cada chat
	for phoneNumber, fks := range fksMap {
		if fks == nil {
			log.Printf("WARNING: FK is nil for phone %s, skipping", phoneNumber)
			continue
		}
		if fks.ContactID == 0 || fks.ConversationID == 0 {
			log.Printf("WARNING: Invalid FK for phone %s: contact_id=%d, conversation_id=%d, skipping", 
				phoneNumber, fks.ContactID, fks.ConversationID)
			continue
		}
		select {
		case <-s.stopChan:
			return nil
		default:
		}

		// Verificar se o chat ainda tem mensagens (pode ter mudado)
		if !chatsWithMessages[phoneNumber] {
			continue
		}

		chat, exists := chatMap[phoneNumber]
		if !exists {
			continue
		}

		chatID := chat.WAChatID
		if chatID == "" {
			chatID = chat.WAChatLID
		}
		if chatID == "" {
			continue
		}

		if err := s.syncChatMessages(chatID, fks, inboxID, chatwootUser); err != nil {
			log.Printf("Error syncing messages for chat %s: %v", chatID, err)
			// Continue com próximo chat
		}
	}

	return nil
}

func (s *Service) syncChatMessages(
	chatID string,
	fks *models.ChatwootFKs,
	inboxID int,
	chatwootUser *models.ChatwootUser,
) error {
	// Buscar todas as mensagens do chat
	messages, err := s.uazapi.GetAllMessages(chatID, s.cfg.Sync.LimitMessages)
	if err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	log.Printf("Processing %d total messages for chat %s", len(messages), chatID)

	// Verificar mensagens existentes
	sourceIDs := make([]string, 0, len(messages))
	for _, msg := range messages {
		sourceIDs = append(sourceIDs, fmt.Sprintf("WAID:%s", msg.MessageID))
	}

	existing, err := s.chatwoot.CheckExistingMessages(sourceIDs, fks.ConversationID)
	if err != nil {
		return fmt.Errorf("failed to check existing messages: %w", err)
	}

	log.Printf("Found %d existing messages out of %d total for chat %s", len(existing), len(messages), chatID)

	// Filtrar apenas mensagens novas
	newMessages := make([]models.ChatwootMessage, 0)
	var lastTimestamp int64

	for _, msg := range messages {
		sourceID := fmt.Sprintf("WAID:%s", msg.MessageID)
		if existing[sourceID] {
			continue
		}

		content := s.extractMessageContent(msg)
		if content == "" {
			continue // Pular mensagens vazias
		}

		messageType := "0" // incoming
		senderType := "Contact"
		senderID := fks.ContactID

		if msg.FromMe {
			messageType = "1" // outgoing
			senderType = chatwootUser.UserType
			senderID = chatwootUser.UserID
		}

		if msg.MessageTimestamp > lastTimestamp {
			lastTimestamp = msg.MessageTimestamp
		}

		newMessages = append(newMessages, models.ChatwootMessage{
			Content:          content,
			ConversationID:   fks.ConversationID,
			MessageType:     messageType,
			SenderType:       senderType,
			SenderID:         senderID,
			SourceID:         sourceID,
			MessageTimestamp: msg.MessageTimestamp,
		})
	}

	log.Printf("Prepared %d new messages to insert for chat %s", len(newMessages), chatID)

	if len(newMessages) == 0 {
		log.Printf("No new messages to insert for chat %s", chatID)
		return nil
	}

	// Ordenar mensagens por timestamp (mais antigas primeiro) para garantir ordem cronológica
	sort.Slice(newMessages, func(i, j int) bool {
		return newMessages[i].MessageTimestamp < newMessages[j].MessageTimestamp
	})

	log.Printf("Messages sorted by timestamp (oldest first) for chat %s", chatID)

	// Inserir mensagens diretamente no banco
	batchSize := s.cfg.Sync.BatchSize
	totalInserted := 0
	for i := 0; i < len(newMessages); i += batchSize {
		end := i + batchSize
		if end > len(newMessages) {
			end = len(newMessages)
		}

		batch := newMessages[i:end]
		inserted, err := s.chatwoot.InsertMessages(batch, inboxID)
		if err != nil {
			return fmt.Errorf("failed to insert messages: %w", err)
		}

		totalInserted += inserted
		log.Printf("Inserted batch %d-%d: %d messages (total: %d/%d) for conversation %d", 
			i+1, end, inserted, totalInserted, len(newMessages), fks.ConversationID)
	}
	log.Printf("Successfully inserted %d messages for conversation %d", 
		totalInserted, fks.ConversationID)

	// Atualizar última atividade
	if lastTimestamp > 0 {
		if err := s.chatwoot.UpdateConversationLastActivity(fks.ConversationID, lastTimestamp); err != nil {
			log.Printf("Warning: failed to update conversation activity: %v", err)
		}
	}

	return nil
}

func (s *Service) normalizePhoneNumber(phone string) string {
	// Remove espaços e caracteres especiais
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")

	// Garantir que começa com +
	if !strings.HasPrefix(phone, "+") {
		// Se não tem +, assumir que é número brasileiro
		if strings.HasPrefix(phone, "55") {
			phone = "+" + phone
		} else if len(phone) >= 10 {
			phone = "+55" + phone
		} else {
			return ""
		}
	}

	return phone
}

func (s *Service) buildIdentifier(phoneNumber string) string {
	// Remove o + e adiciona @s.whatsapp.net
	number := strings.TrimPrefix(phoneNumber, "+")
	return number + "@s.whatsapp.net"
}

func (s *Service) getContactName(chat models.UAZAPIChat) string {
	if chat.WAContactName != "" {
		return chat.WAContactName
	}
	if chat.WAName != "" {
		return chat.WAName
	}
	if chat.Name != "" {
		return chat.Name
	}
	return chat.Phone
}

func (s *Service) extractMessageContent(msg models.UAZAPIMessage) string {
	if msg.Text != "" {
		return msg.Text
	}
	return msg.MessageType
}

func (s *Service) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

