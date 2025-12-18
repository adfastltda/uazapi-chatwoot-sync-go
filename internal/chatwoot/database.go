package chatwoot

import (
	"chatwoot-sync-go/internal/config"
	"chatwoot-sync-go/internal/models"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type Database struct {
	db *sql.DB
	cfg *config.Config
}

func NewDatabase(cfg *config.Config) (*Database, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Chatwoot.DB.Host,
		cfg.Chatwoot.DB.Port,
		cfg.Chatwoot.DB.User,
		cfg.Chatwoot.DB.Password,
		cfg.Chatwoot.DB.Name,
		cfg.Chatwoot.DB.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Connected to Chatwoot database successfully")

	return &Database{
		db:  db,
		cfg: cfg,
	}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// ListInboxes lista todos os inboxes disponíveis para debug
func (d *Database) ListInboxes() ([]map[string]interface{}, error) {
	query := `SELECT id, name, inbox_type FROM inboxes WHERE account_id = $1 ORDER BY id`
	rows, err := d.db.Query(query, d.cfg.Chatwoot.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list inboxes: %w", err)
	}
	defer rows.Close()

	var inboxes []map[string]interface{}
	for rows.Next() {
		var id int
		var name, inboxType string
		if err := rows.Scan(&id, &name, &inboxType); err != nil {
			return nil, fmt.Errorf("failed to scan inbox: %w", err)
		}
		inboxes = append(inboxes, map[string]interface{}{
			"id":         id,
			"name":       name,
			"inbox_type": inboxType,
		})
	}

	return inboxes, nil
}

// GetInbox busca o inbox pelo nome ou ID, ou usa o primeiro disponível
func (d *Database) GetInbox() (int, error) {
	var inboxID int
	var accountID int
	
	log.Printf("Searching for inbox: account_id=%d, inbox_id=%d, inbox_name='%s'", 
		d.cfg.Chatwoot.AccountID, d.cfg.Chatwoot.InboxID, d.cfg.Chatwoot.InboxName)
	
	// Primeiro, lista todos os inboxes para debug
	inboxes, listErr := d.ListInboxes()
	if listErr == nil && len(inboxes) > 0 {
		log.Printf("Available inboxes for account %d:", d.cfg.Chatwoot.AccountID)
		for _, inbox := range inboxes {
			log.Printf("  - ID: %v, Name: %v, Type: %v", inbox["id"], inbox["name"], inbox["inbox_type"])
		}
	}
	
	// Se inbox_id foi fornecido, verifica se existe (sem filtrar por account_id primeiro)
	if d.cfg.Chatwoot.InboxID > 0 {
		// Primeiro verifica se o inbox existe (qualquer conta)
		var tempID, tempAccountID int
		checkQuery := `SELECT id, account_id FROM inboxes WHERE id = $1 LIMIT 1`
		err := d.db.QueryRow(checkQuery, d.cfg.Chatwoot.InboxID).Scan(&tempID, &tempAccountID)
		if err == nil {
			if tempAccountID == d.cfg.Chatwoot.AccountID {
				log.Printf("Found inbox ID %d in account %d", tempID, tempAccountID)
				return tempID, nil
			} else {
				log.Printf("Warning: Inbox ID %d exists but belongs to account %d (expected %d)", 
					tempID, tempAccountID, d.cfg.Chatwoot.AccountID)
			}
		}
		
		// Agora tenta com account_id
		query := `SELECT id FROM inboxes WHERE account_id = $1 AND id = $2 LIMIT 1`
		err = d.db.QueryRow(query, d.cfg.Chatwoot.AccountID, d.cfg.Chatwoot.InboxID).Scan(&inboxID)
		if err == nil {
			log.Printf("Found inbox by ID: %d", inboxID)
			return inboxID, nil
		}
		if err == sql.ErrNoRows {
			log.Printf("Inbox ID %d not found for account %d, trying by name...", d.cfg.Chatwoot.InboxID, d.cfg.Chatwoot.AccountID)
		} else {
			log.Printf("Error querying inbox by ID: %v", err)
		}
	}
	
	// Se não encontrou por ID, tenta buscar por nome
	if d.cfg.Chatwoot.InboxName != "" {
		query := `SELECT id FROM inboxes WHERE account_id = $1 AND name = $2 LIMIT 1`
		err := d.db.QueryRow(query, d.cfg.Chatwoot.AccountID, d.cfg.Chatwoot.InboxName).Scan(&inboxID)
		if err == nil {
			log.Printf("Found inbox by name '%s': %d", d.cfg.Chatwoot.InboxName, inboxID)
			return inboxID, nil
		}
		if err == sql.ErrNoRows {
			log.Printf("Inbox name '%s' not found for account %d, trying first available...", d.cfg.Chatwoot.InboxName, d.cfg.Chatwoot.AccountID)
		} else {
			log.Printf("Error querying inbox by name: %v", err)
		}
	}
	
	// Se não encontrou, tenta buscar qualquer inbox da conta
	query := `SELECT id FROM inboxes WHERE account_id = $1 ORDER BY id LIMIT 1`
	err := d.db.QueryRow(query, d.cfg.Chatwoot.AccountID).Scan(&inboxID)
	if err == nil {
		log.Printf("Warning: Using first available inbox (ID: %d) from account %d", inboxID, d.cfg.Chatwoot.AccountID)
		return inboxID, nil
	}
	if err != sql.ErrNoRows {
		log.Printf("Error querying first inbox: %v", err)
	}
	
	// Se ainda não encontrou, verifica se há inboxes em outras contas
	if d.cfg.Chatwoot.InboxID > 0 {
		checkAllQuery := `SELECT id, account_id FROM inboxes WHERE id = $1 LIMIT 1`
		err := d.db.QueryRow(checkAllQuery, d.cfg.Chatwoot.InboxID).Scan(&inboxID, &accountID)
		if err == nil {
			return 0, fmt.Errorf("inbox ID %d exists but belongs to account %d (configured account: %d). Please update CHATWOOT_ACCOUNT_ID or use the correct inbox", 
				inboxID, accountID, d.cfg.Chatwoot.AccountID)
		}
	}
	
	return 0, fmt.Errorf("no inbox found for account_id=%d (inbox_id=%d, inbox_name='%s'). Please create an inbox in Chatwoot first or check the configuration", 
		d.cfg.Chatwoot.AccountID, d.cfg.Chatwoot.InboxID, d.cfg.Chatwoot.InboxName)
}

// GetChatwootUser busca o usuário do token
func (d *Database) GetChatwootUser(token string) (*models.ChatwootUser, error) {
	var user models.ChatwootUser
	query := `SELECT owner_type AS user_type, owner_id AS user_id FROM access_tokens WHERE token = $1 LIMIT 1`
	
	err := d.db.QueryRow(query, token).Scan(&user.UserType, &user.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chatwoot user: %w", err)
	}

	return &user, nil
}

// CreateContactsAndConversations cria contatos e conversas usando CTE
func (d *Database) CreateContactsAndConversations(
	contacts []models.ChatwootContact,
	inboxID int,
) (map[string]*models.ChatwootFKs, error) {
	if len(contacts) == 0 {
		return make(map[string]*models.ChatwootFKs), nil
	}

	// Construir VALUES para a CTE
	var values []string
	var args []interface{}
	argIndex := 3 // $1 = account_id, $2 = inbox_id, $3+ = valores

	for _, contact := range contacts {
		values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d)", argIndex, argIndex+1, argIndex+2, argIndex+3))
		args = append(args, contact.PhoneNumber, contact.Name, contact.FirstTimestamp, contact.LastTimestamp)
		argIndex += 4
	}

	// Query completa com CTE
	query := fmt.Sprintf(`
		WITH
			phone_number AS (
				SELECT phone_number, contact_name, created_at::BIGINT, last_activity_at::BIGINT FROM (
					VALUES %s
				) as t (phone_number, contact_name, created_at, last_activity_at)
			),
			only_new_phone_number AS (
				SELECT * FROM phone_number
				WHERE phone_number NOT IN (
					SELECT phone_number
					FROM contacts
						JOIN contact_inboxes ci ON ci.contact_id = contacts.id AND ci.inbox_id = $2
						JOIN conversations con ON con.contact_inbox_id = ci.id 
							AND con.account_id = $1
							AND con.inbox_id = $2
							AND con.contact_id = contacts.id
					WHERE contacts.account_id = $1
				)
			),
			new_contact AS (
				INSERT INTO contacts (name, phone_number, account_id, identifier, created_at, updated_at)
				SELECT 
					COALESCE(NULLIF(TRIM(p.contact_name), ''), REPLACE(p.phone_number, '+', '')) as name,
					p.phone_number,
					$1,
					CONCAT(REPLACE(p.phone_number, '+', ''), '@s.whatsapp.net') as identifier,
					to_timestamp(CASE WHEN p.created_at > 10000000000 THEN p.created_at / 1000 ELSE p.created_at END),
					to_timestamp(CASE WHEN p.last_activity_at > 10000000000 THEN p.last_activity_at / 1000 ELSE p.last_activity_at END)
				FROM only_new_phone_number AS p
				ON CONFLICT(identifier, account_id) 
				DO UPDATE SET 
					name = COALESCE(NULLIF(TRIM(EXCLUDED.name), ''), contacts.name),
					updated_at = EXCLUDED.updated_at
				RETURNING id, phone_number, created_at, updated_at
			),
			new_contact_inbox AS (
				INSERT INTO contact_inboxes (contact_id, inbox_id, source_id, created_at, updated_at)
				SELECT new_contact.id, $2, gen_random_uuid(), new_contact.created_at, new_contact.updated_at
				FROM new_contact 
				ON CONFLICT (contact_id, inbox_id) DO UPDATE SET updated_at = NOW()
				RETURNING id, contact_id, created_at, updated_at
			),
			new_conversation AS (
				INSERT INTO conversations (account_id, inbox_id, status, contact_id,
					contact_inbox_id, uuid, last_activity_at, created_at, updated_at)
				SELECT $1, $2, 0, new_contact_inbox.contact_id, new_contact_inbox.id, gen_random_uuid(),
					new_contact_inbox.updated_at, new_contact_inbox.created_at, new_contact_inbox.updated_at
				FROM new_contact_inbox
				WHERE NOT EXISTS (
					SELECT 1 FROM conversations 
					WHERE contact_inbox_id = new_contact_inbox.id
						AND account_id = $1
						AND inbox_id = $2
				)
				RETURNING id, contact_id
			)
			SELECT new_contact.phone_number, new_conversation.contact_id, new_conversation.id AS conversation_id
			FROM new_conversation 
			JOIN new_contact ON new_conversation.contact_id = new_contact.id
		UNION
			SELECT p.phone_number, c.id contact_id, con.id conversation_id
			FROM phone_number p
			JOIN contacts c ON c.phone_number = p.phone_number
			JOIN contact_inboxes ci ON ci.contact_id = c.id AND ci.inbox_id = $2
			JOIN conversations con ON con.contact_inbox_id = ci.id AND con.account_id = $1
				AND con.inbox_id = $2 AND con.contact_id = c.id
	`, strings.Join(values, ","))

	// Preparar argumentos: account_id, inbox_id, depois os valores
	args = append([]interface{}{d.cfg.Chatwoot.AccountID, inboxID}, args...)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*models.ChatwootFKs)
	for rows.Next() {
		var fk models.ChatwootFKs
		if err := rows.Scan(&fk.PhoneNumber, &fk.ContactID, &fk.ConversationID); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result[fk.PhoneNumber] = &fk
	}

	// Atualizar nomes dos contatos existentes que não têm nome ou têm apenas o número
	// Construir VALUES apenas com phone_number e contact_name
	var updateValues []string
	var updateArgs []interface{}
	updateArgIndex := 2 // $1 = account_id, $2+ = valores
	
	for i := 0; i < len(contacts); i++ {
		updateValues = append(updateValues, fmt.Sprintf("($%d, $%d)", updateArgIndex, updateArgIndex+1))
		updateArgs = append(updateArgs, contacts[i].PhoneNumber, contacts[i].Name)
		updateArgIndex += 2
	}
	
	if len(updateValues) > 0 {
		updateQuery := fmt.Sprintf(`
			UPDATE contacts c
			SET name = COALESCE(NULLIF(TRIM(p.contact_name), ''), c.name)
			FROM (
				VALUES %s
			) as p (phone_number, contact_name)
			WHERE c.phone_number = p.phone_number 
				AND c.account_id = $1
				AND NULLIF(TRIM(p.contact_name), '') IS NOT NULL
				AND (c.name IS NULL OR c.name = '' OR c.name = REPLACE(c.phone_number, '+', ''))
		`, strings.Join(updateValues, ","))
		
		updateArgs = append([]interface{}{d.cfg.Chatwoot.AccountID}, updateArgs...)
		_, err = d.db.Exec(updateQuery, updateArgs...)
		if err != nil {
			log.Printf("Warning: failed to update existing contact names: %v", err)
			// Não retornar erro, apenas logar
		}
	}

	return result, nil
}

// CheckExistingMessages verifica quais mensagens já existem
func (d *Database) CheckExistingMessages(sourceIDs []string, conversationID int) (map[string]bool, error) {
	if len(sourceIDs) == 0 {
		return make(map[string]bool), nil
	}

	// Usar pq.Array para converter []string em array PostgreSQL
	query := `SELECT source_id FROM messages WHERE source_id = ANY($1) AND conversation_id = $2`
	rows, err := d.db.Query(query, pq.Array(sourceIDs), conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing messages: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]bool)
	count := 0
	for rows.Next() {
		var sourceID string
		if err := rows.Scan(&sourceID); err != nil {
			return nil, fmt.Errorf("failed to scan source_id: %w", err)
		}
		existing[sourceID] = true
		count++
	}

	log.Printf("CheckExistingMessages: Found %d existing messages out of %d checked for conversation %d", 
		count, len(sourceIDs), conversationID)

	return existing, nil
}

// InsertMessages insere mensagens em lote
func (d *Database) InsertMessages(messages []models.ChatwootMessage, inboxID int) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	log.Printf("InsertMessages: Preparing to insert %d messages for conversation %d", 
		len(messages), messages[0].ConversationID)

	// Construir VALUES
	var values []string
	var args []interface{}
	argIndex := 1

		for i, msg := range messages {
		// Verificar se o timestamp está em milissegundos ou segundos
		// Timestamps do WhatsApp geralmente são em segundos (Unix timestamp)
		// Se o valor for maior que 10^10, provavelmente está em milissegundos
		// Timestamps Unix atuais (2024-2025) estão entre 1700000000 e 1800000000 segundos
		var timestampSeconds int64
		
		// Log detalhado para primeira mensagem
		if i == 0 {
			log.Printf("InsertMessages: Raw timestamp value: %d", msg.MessageTimestamp)
		}
		
		if msg.MessageTimestamp > 10000000000 {
			// Está em milissegundos, converter para segundos
			timestampSeconds = msg.MessageTimestamp / 1000
			if i == 0 {
				log.Printf("InsertMessages: Timestamp detected as milliseconds (%d), converting to seconds (%d)", 
					msg.MessageTimestamp, timestampSeconds)
			}
		} else if msg.MessageTimestamp > 1000000000 {
			// Está em segundos (valor entre 1 bilhão e 10 bilhões = timestamp Unix válido)
			timestampSeconds = msg.MessageTimestamp
			if i == 0 {
				log.Printf("InsertMessages: Timestamp detected as seconds (%d), using directly", timestampSeconds)
			}
		} else {
			// Valor muito pequeno, pode estar em segundos mas muito antigo, ou pode ser um erro
			// Vamos assumir que está em segundos mesmo assim
			timestampSeconds = msg.MessageTimestamp
			if i == 0 {
				log.Printf("InsertMessages: WARNING: Timestamp value is very small (%d), assuming seconds", timestampSeconds)
			}
		}
		
		// Validação adicional: se o timestamp resultante for muito grande (mais de 10 bilhões),
		// provavelmente ainda está em milissegundos - dividir novamente
		if timestampSeconds > 10000000000 {
			log.Printf("InsertMessages: WARNING: Final timestamp is still very large (%d), dividing by 1000 again", timestampSeconds)
			timestampSeconds = timestampSeconds / 1000
		}
		
		// Validação final: timestamps Unix válidos estão entre 1000000000 (2001) e 2000000000 (2033)
		// Se estiver fora desse range, há um problema
		if timestampSeconds < 1000000000 || timestampSeconds > 2000000000 {
			log.Printf("InsertMessages: ERROR: Invalid timestamp value %d (expected range: 1000000000-2000000000)", timestampSeconds)
			// Tentar corrigir: se muito grande, dividir por 1000
			if timestampSeconds > 2000000000 {
				original := timestampSeconds
				timestampSeconds = timestampSeconds / 1000
				log.Printf("InsertMessages: Attempting correction: %d -> %d", original, timestampSeconds)
			}
		}
		
		values = append(values, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, FALSE, 0, $%d, $%d, $%d, to_timestamp($%d), to_timestamp($%d))",
			argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4, argIndex+5, argIndex+6, argIndex+7, argIndex+8, argIndex+9, argIndex+10,
		))
		args = append(args,
			msg.Content,
			msg.Content, // processed_message_content (duplicado)
			d.cfg.Chatwoot.AccountID,
			inboxID,
			msg.ConversationID,
			msg.MessageType,
			msg.SenderType,
			msg.SenderID,
			msg.SourceID,
			timestampSeconds,
			timestampSeconds,
		)
		argIndex += 11

		// Log primeira e última mensagem para debug
		if i == 0 || i == len(messages)-1 {
			log.Printf("InsertMessages: Message %d/%d - source_id=%s, original_timestamp=%d, final_timestamp=%d, content_preview=%.50s", 
				i+1, len(messages), msg.SourceID, msg.MessageTimestamp, timestampSeconds, msg.Content)
		}
	}

	query := fmt.Sprintf(`
		INSERT INTO messages (
			content, processed_message_content, account_id, inbox_id, conversation_id,
			message_type, private, content_type, sender_type, sender_id, source_id,
			created_at, updated_at
		) VALUES %s
	`, strings.Join(values, ","))

	result, err := d.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to insert messages: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	log.Printf("InsertMessages: Successfully inserted %d messages for conversation %d", 
		count, messages[0].ConversationID)

	return int(count), nil
}

// UpdateConversationLastActivity atualiza a última atividade da conversa
func (d *Database) UpdateConversationLastActivity(conversationID int, timestamp int64) error {
	// Verificar se o timestamp está em milissegundos ou segundos
	var timestampSeconds int64
	if timestamp > 10000000000 {
		// Está em milissegundos, converter para segundos
		timestampSeconds = timestamp / 1000
	} else {
		// Já está em segundos
		timestampSeconds = timestamp
	}
	
	query := `
		UPDATE conversations
		SET 
			last_activity_at = GREATEST(last_activity_at, to_timestamp($1)),
			updated_at = NOW()
		WHERE id = $2
	`

	_, err := d.db.Exec(query, timestampSeconds, conversationID)
	if err != nil {
		return fmt.Errorf("failed to update conversation activity: %w", err)
	}

	return nil
}

