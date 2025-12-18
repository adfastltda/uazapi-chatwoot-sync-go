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
				WHERE NOT EXISTS (
					SELECT 1 FROM contacts 
					WHERE identifier = CONCAT(REPLACE(p.phone_number, '+', ''), '@s.whatsapp.net')
						AND account_id = $1
				)
				RETURNING id, phone_number, created_at, updated_at
			),
			new_contact_inbox AS (
				INSERT INTO contact_inboxes (contact_id, inbox_id, source_id, created_at, updated_at)
				SELECT new_contact.id, $2, gen_random_uuid(), new_contact.created_at, new_contact.updated_at
				FROM new_contact 
				WHERE NOT EXISTS (
					SELECT 1 FROM contact_inboxes 
					WHERE contact_id = new_contact.id 
						AND inbox_id = $2
				)
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
		UNION ALL
			SELECT p.phone_number, c.id contact_id, COALESCE(con.id, 0) conversation_id
			FROM phone_number p
			JOIN contacts c ON c.phone_number = p.phone_number AND c.account_id = $1
			LEFT JOIN contact_inboxes ci ON ci.contact_id = c.id AND ci.inbox_id = $2
			LEFT JOIN conversations con ON con.contact_inbox_id = ci.id 
				AND con.account_id = $1
				AND con.inbox_id = $2 
				AND con.contact_id = c.id
			WHERE NOT EXISTS (
				SELECT 1 FROM new_conversation WHERE new_conversation.contact_id = c.id
			)
	`, strings.Join(values, ","))

	// Preparar argumentos: account_id, inbox_id, depois os valores
	args = append([]interface{}{d.cfg.Chatwoot.AccountID, inboxID}, args...)

	log.Printf("CreateContactsAndConversations: Executing query for %d contacts (account_id=%d, inbox_id=%d)", 
		len(contacts), d.cfg.Chatwoot.AccountID, inboxID)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		log.Printf("CreateContactsAndConversations: Query failed: %v", err)
		log.Printf("CreateContactsAndConversations: Query was: %s", query)
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*models.ChatwootFKs)
	rowCount := 0
	for rows.Next() {
		var fk models.ChatwootFKs
		if err := rows.Scan(&fk.PhoneNumber, &fk.ContactID, &fk.ConversationID); err != nil {
			log.Printf("CreateContactsAndConversations: Failed to scan row: %v", err)
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result[fk.PhoneNumber] = &fk
		rowCount++
		log.Printf("CreateContactsAndConversations: Found FK for phone %s: contact_id=%d, conversation_id=%d", 
			fk.PhoneNumber, fk.ContactID, fk.ConversationID)
	}
	
	log.Printf("CreateContactsAndConversations: Query returned %d rows (expected %d contacts)", rowCount, len(contacts))
	
	if rowCount < len(contacts) {
		// Identificar quais contatos não foram retornados
		returnedPhones := make(map[string]bool)
		for phone := range result {
			returnedPhones[phone] = true
		}
		
		missingPhones := make([]string, 0)
		for _, contact := range contacts {
			if !returnedPhones[contact.PhoneNumber] {
				missingPhones = append(missingPhones, contact.PhoneNumber)
			}
		}
		
		if len(missingPhones) > 0 {
			log.Printf("CreateContactsAndConversations: WARNING - %d contacts not returned: %v", len(missingPhones), missingPhones)
			log.Printf("CreateContactsAndConversations: These contacts may exist but don't have conversations, or query failed to match them")
			
			// Criar um mapa para buscar informações do contato
			contactMap := make(map[string]models.ChatwootContact)
			for _, contact := range contacts {
				contactMap[contact.PhoneNumber] = contact
			}
			
			// Tentar buscar manualmente os contatos faltantes
			for _, missingPhone := range missingPhones {
				log.Printf("CreateContactsAndConversations: Attempting to find contact manually for phone: %s", missingPhone)
				manualFK, err := d.findContactManually(missingPhone, inboxID)
				if err != nil {
					log.Printf("CreateContactsAndConversations: Failed to find contact manually for %s: %v", missingPhone, err)
				} else if manualFK != nil {
					result[missingPhone] = manualFK
					log.Printf("CreateContactsAndConversations: Found contact manually for %s: contact_id=%d, conversation_id=%d", 
						missingPhone, manualFK.ContactID, manualFK.ConversationID)
				} else {
					// Contato não existe - criar agora
					log.Printf("CreateContactsAndConversations: Contact %s does not exist, creating it now", missingPhone)
					contactInfo, exists := contactMap[missingPhone]
					if !exists {
						log.Printf("CreateContactsAndConversations: WARNING - Contact info not found for %s, skipping", missingPhone)
						continue
					}
					
					createdFK, err := d.createContactAndConversation(contactInfo, inboxID)
					if err != nil {
						log.Printf("CreateContactsAndConversations: Failed to create contact for %s: %v", missingPhone, err)
					} else {
						result[missingPhone] = createdFK
						log.Printf("CreateContactsAndConversations: Created contact for %s: contact_id=%d, conversation_id=%d", 
							missingPhone, createdFK.ContactID, createdFK.ConversationID)
					}
				}
			}
		}
	}
	
	if rowCount == 0 && len(contacts) > 0 {
		log.Printf("CreateContactsAndConversations: WARNING - No rows returned but %d contacts provided", len(contacts))
		log.Printf("CreateContactsAndConversations: This might mean all contacts already exist or query failed silently")
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

// findContactManually busca um contato manualmente quando a query CTE não o retorna
func (d *Database) findContactManually(phoneNumber string, inboxID int) (*models.ChatwootFKs, error) {
	log.Printf("findContactManually: Searching for contact with phone_number='%s', account_id=%d, inbox_id=%d", 
		phoneNumber, d.cfg.Chatwoot.AccountID, inboxID)
	
	// Primeiro, tentar buscar por phone_number
	query := `
		SELECT c.id, con.id
		FROM contacts c
		LEFT JOIN contact_inboxes ci ON ci.contact_id = c.id AND ci.inbox_id = $2
		LEFT JOIN conversations con ON con.contact_inbox_id = ci.id 
			AND con.account_id = $1
			AND con.inbox_id = $2
			AND con.contact_id = c.id
		WHERE c.phone_number = $3
			AND c.account_id = $1
		LIMIT 1
	`
	
	var contactID, conversationID sql.NullInt64
	err := d.db.QueryRow(query, d.cfg.Chatwoot.AccountID, inboxID, phoneNumber).Scan(&contactID, &conversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Se não encontrou por phone_number, tentar buscar por identifier
			identifier := strings.TrimPrefix(phoneNumber, "+") + "@s.whatsapp.net"
			log.Printf("findContactManually: Contact not found by phone_number, trying identifier='%s'", identifier)
			
			queryByIdentifier := `
				SELECT c.id, con.id
				FROM contacts c
				LEFT JOIN contact_inboxes ci ON ci.contact_id = c.id AND ci.inbox_id = $2
				LEFT JOIN conversations con ON con.contact_inbox_id = ci.id 
					AND con.account_id = $1
					AND con.inbox_id = $2
					AND con.contact_id = c.id
				WHERE c.identifier = $3
					AND c.account_id = $1
				LIMIT 1
			`
			
			err = d.db.QueryRow(queryByIdentifier, d.cfg.Chatwoot.AccountID, inboxID, identifier).Scan(&contactID, &conversationID)
			if err != nil {
				if err == sql.ErrNoRows {
					log.Printf("findContactManually: Contact with phone_number='%s' or identifier='%s' not found in database", phoneNumber, identifier)
					return nil, nil // Contato não existe
				}
				log.Printf("findContactManually: Error querying contact by identifier: %v", err)
				return nil, fmt.Errorf("failed to query contact: %w", err)
			}
		} else {
			log.Printf("findContactManually: Error querying contact: %v", err)
			return nil, fmt.Errorf("failed to query contact: %w", err)
		}
	}
	
	if !contactID.Valid {
		log.Printf("findContactManually: ContactID is not valid for phone_number='%s'", phoneNumber)
		return nil, nil // Contato não encontrado
	}
	
	log.Printf("findContactManually: Found contact_id=%d for phone_number='%s'", contactID.Int64, phoneNumber)
	
	fk := &models.ChatwootFKs{
		PhoneNumber:   phoneNumber,
		ContactID:     int(contactID.Int64),
		ConversationID: 0,
	}
	
	if conversationID.Valid {
		fk.ConversationID = int(conversationID.Int64)
		log.Printf("findContactManually: Contact %d already has conversation_id=%d", fk.ContactID, fk.ConversationID)
	} else {
			// Contato existe mas não tem conversa - criar conversa
			log.Printf("findContactManually: Contact %d exists but has no conversation, creating one", fk.ContactID)
			
			// Buscar ou criar contact_inbox
			var contactInboxID sql.NullInt64
			ciQuery := `SELECT id FROM contact_inboxes WHERE contact_id = $1 AND inbox_id = $2 LIMIT 1`
			err = d.db.QueryRow(ciQuery, fk.ContactID, inboxID).Scan(&contactInboxID)
			if err != nil && err != sql.ErrNoRows {
				return nil, fmt.Errorf("failed to query contact_inbox: %w", err)
			}
			
			if !contactInboxID.Valid {
				// Criar contact_inbox
				ciInsert := `INSERT INTO contact_inboxes (contact_id, inbox_id, source_id, created_at, updated_at) 
					VALUES ($1, $2, gen_random_uuid(), NOW(), NOW()) RETURNING id`
				err = d.db.QueryRow(ciInsert, fk.ContactID, inboxID).Scan(&contactInboxID)
				if err != nil {
					return nil, fmt.Errorf("failed to create contact_inbox: %w", err)
				}
				log.Printf("findContactManually: Created contact_inbox %d for contact %d", contactInboxID.Int64, fk.ContactID)
			}
			
			// Verificar se já existe conversa
			var existingConvID sql.NullInt64
			convCheck := `SELECT id FROM conversations WHERE contact_inbox_id = $1 AND account_id = $2 AND inbox_id = $3 LIMIT 1`
			err = d.db.QueryRow(convCheck, contactInboxID.Int64, d.cfg.Chatwoot.AccountID, inboxID).Scan(&existingConvID)
			if err != nil && err != sql.ErrNoRows {
				return nil, fmt.Errorf("failed to check conversation: %w", err)
			}
			
			if existingConvID.Valid {
				fk.ConversationID = int(existingConvID.Int64)
				log.Printf("findContactManually: Found existing conversation %d for contact %d", fk.ConversationID, fk.ContactID)
			} else {
				// Criar conversa
				convInsert := `INSERT INTO conversations (account_id, inbox_id, status, contact_id, contact_inbox_id, uuid, last_activity_at, created_at, updated_at)
					VALUES ($1, $2, 0, $3, $4, gen_random_uuid(), NOW(), NOW(), NOW()) RETURNING id`
				err = d.db.QueryRow(convInsert, d.cfg.Chatwoot.AccountID, inboxID, fk.ContactID, contactInboxID.Int64).Scan(&conversationID)
				if err != nil {
					return nil, fmt.Errorf("failed to create conversation: %w", err)
				}
				
				fk.ConversationID = int(conversationID.Int64)
				log.Printf("findContactManually: Created conversation %d for contact %d", fk.ConversationID, fk.ContactID)
		}
	}
	
	log.Printf("findContactManually: Returning FK for phone_number='%s': contact_id=%d, conversation_id=%d", 
		phoneNumber, fk.ContactID, fk.ConversationID)
	return fk, nil
}

// createContactAndConversation cria um contato e sua conversa quando ele não existe
func (d *Database) createContactAndConversation(contact models.ChatwootContact, inboxID int) (*models.ChatwootFKs, error) {
	log.Printf("createContactAndConversation: Creating contact for phone_number='%s', name='%s'", contact.PhoneNumber, contact.Name)
	
	// Converter timestamps
	createdAt := contact.FirstTimestamp
	if createdAt > 10000000000 {
		createdAt = createdAt / 1000
	}
	updatedAt := contact.LastTimestamp
	if updatedAt > 10000000000 {
		updatedAt = updatedAt / 1000
	}
	
	// Criar contato
	contactName := contact.Name
	if contactName == "" {
		contactName = strings.TrimPrefix(contact.PhoneNumber, "+")
	}
	
	identifier := contact.Identifier
	if identifier == "" {
		identifier = strings.TrimPrefix(contact.PhoneNumber, "+") + "@s.whatsapp.net"
	}
	
	// Verificar se o contato já existe pelo identifier (devido à constraint única)
	var existingContactID sql.NullInt64
	checkQuery := `SELECT id FROM contacts WHERE identifier = $1 AND account_id = $2 LIMIT 1`
	err := d.db.QueryRow(checkQuery, identifier, d.cfg.Chatwoot.AccountID).Scan(&existingContactID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing contact: %w", err)
	}
	
	var contactID int64
	if existingContactID.Valid {
		// Contato já existe pelo identifier, usar o existente
		contactID = existingContactID.Int64
		log.Printf("createContactAndConversation: Contact already exists with identifier='%s', using contact_id=%d", identifier, contactID)
		
		// Atualizar phone_number se necessário
		updateQuery := `UPDATE contacts SET phone_number = $1, name = COALESCE(NULLIF(TRIM($2), ''), name) WHERE id = $3`
		_, err = d.db.Exec(updateQuery, contact.PhoneNumber, contactName, contactID)
		if err != nil {
			log.Printf("createContactAndConversation: Warning - failed to update contact phone_number: %v", err)
		}
	} else {
		// Criar novo contato
		contactInsert := `
			INSERT INTO contacts (name, phone_number, account_id, identifier, created_at, updated_at)
			VALUES ($1, $2, $3, $4, to_timestamp($5), to_timestamp($6))
			RETURNING id
		`
		err = d.db.QueryRow(contactInsert, contactName, contact.PhoneNumber, d.cfg.Chatwoot.AccountID, identifier, createdAt, updatedAt).Scan(&contactID)
		if err != nil {
			return nil, fmt.Errorf("failed to create contact: %w", err)
		}
		log.Printf("createContactAndConversation: Created contact_id=%d", contactID)
	}
	
	// Criar contact_inbox
	var contactInboxID int64
	ciInsert := `INSERT INTO contact_inboxes (contact_id, inbox_id, source_id, created_at, updated_at) 
		VALUES ($1, $2, gen_random_uuid(), to_timestamp($3), to_timestamp($4)) RETURNING id`
	err = d.db.QueryRow(ciInsert, contactID, inboxID, createdAt, updatedAt).Scan(&contactInboxID)
	if err != nil {
		return nil, fmt.Errorf("failed to create contact_inbox: %w", err)
	}
	log.Printf("createContactAndConversation: Created contact_inbox_id=%d", contactInboxID)
	
	// Criar conversa
	var conversationID int64
	convInsert := `INSERT INTO conversations (account_id, inbox_id, status, contact_id, contact_inbox_id, uuid, last_activity_at, created_at, updated_at)
		VALUES ($1, $2, 0, $3, $4, gen_random_uuid(), to_timestamp($5), to_timestamp($6), to_timestamp($7)) RETURNING id`
	err = d.db.QueryRow(convInsert, d.cfg.Chatwoot.AccountID, inboxID, contactID, contactInboxID, updatedAt, createdAt, updatedAt).Scan(&conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	log.Printf("createContactAndConversation: Created conversation_id=%d", conversationID)
	
	return &models.ChatwootFKs{
		PhoneNumber:   contact.PhoneNumber,
		ContactID:     int(contactID),
		ConversationID: int(conversationID),
	}, nil
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

