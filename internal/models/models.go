package models

// UAZAPI Models
type UAZAPIChat struct {
	ID                    string   `json:"id"`
	WAFastID             string   `json:"wa_fastid"`
	WAChatID             string   `json:"wa_chatid"`
	WAChatLID            string   `json:"wa_chatlid"`
	WAArchived           bool     `json:"wa_archived"`
	WAContactName        string   `json:"wa_contactName"`
	WAName               string   `json:"wa_name"`
	Name                 string   `json:"name"`
	Image                string   `json:"image"`
	ImagePreview         string   `json:"imagePreview"`
	WALastMessageTextVote string  `json:"wa_lastMessageTextVote"`
	WALastMessageType    string   `json:"wa_lastMessageType"`
	WALastMsgTimestamp   int64    `json:"wa_lastMsgTimestamp"`
	WALastMessageSender  string   `json:"wa_lastMessageSender"`
	Phone                string   `json:"phone"`
	WAIsGroup            bool     `json:"wa_isGroup"`
	WAUnreadCount        int      `json:"wa_unreadCount"`
}

type UAZAPIMessage struct {
	ID                string `json:"id"`
	MessageID         string `json:"messageid"`
	ChatID            string `json:"chatid"`
	Sender            string `json:"sender"`
	SenderName        string `json:"senderName"`
	IsGroup           bool   `json:"isGroup"`
	FromMe            bool   `json:"fromMe"`
	MessageType       string `json:"messageType"`
	Source            string `json:"source"`
	MessageTimestamp  int64  `json:"messageTimestamp"`
	Status            string `json:"status"`
	Text              string `json:"text"`
	Quoted            string `json:"quoted"`
	FileURL           string `json:"fileURL"`
	SenderPN          string `json:"sender_pn"`
	SenderLID         string `json:"sender_lid"`
}

// UAZAPIMediaResponse representa a resposta do endpoint de download de mídia
type UAZAPIMediaResponse struct {
	Base64Data string `json:"base64Data"`
	Cached     bool   `json:"cached"`
	FileURL    string `json:"fileURL"`
	MimeType   string `json:"mimetype"`
}

type UAZAPIChatsResponse struct {
	Chats     []UAZAPIChat `json:"chats"`
	Pagination struct {
		TotalRecords int  `json:"totalRecords"`
		CurrentPage  int  `json:"currentPage"`
		TotalPages   int  `json:"totalPages"`
		PageSize     int  `json:"pageSize"`
		HasNextPage  bool `json:"hasNextPage"`
	} `json:"pagination"`
}

type UAZAPIMessagesResponse struct {
	ReturnedMessages int            `json:"returnedMessages"`
	Messages         []UAZAPIMessage `json:"messages"`
	Limit            int            `json:"limit"`
	Offset           int            `json:"offset"`
	NextOffset       int            `json:"nextOffset"`
	HasMore          bool           `json:"hasMore"`
}

// Chatwoot Models
type ChatwootContact struct {
	PhoneNumber string
	Name        string
	Identifier  string
	FirstTimestamp int64
	LastTimestamp  int64
}

type ChatwootMessage struct {
	Content         string
	ConversationID  int
	MessageType     string // "0" = incoming, "1" = outgoing
	SenderType      string // "Contact" or "User"
	SenderID        int
	SourceID        string // Format: "WAID:{message_id}"
	MessageTimestamp int64
}

type ChatwootFKs struct {
	PhoneNumber    string
	ContactID      int
	ConversationID int
}

