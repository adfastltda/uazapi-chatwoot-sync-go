# Chatwoot Sync Go

Serviço de sincronização em Go que importa chats e mensagens do WhatsApp da API UAZAPI para o Chatwoot. Este serviço cria automaticamente contatos, conversas e sincroniza mensagens históricas no Chatwoot.

## 📋 Características

- ✅ Sincronização automática de chats e mensagens do WhatsApp
- ✅ Criação automática de contatos e conversas no Chatwoot
- ✅ Processamento em lotes para melhor performance
- ✅ Suporte a Docker e Docker Compose
- ✅ Processamento apenas de mensagens de texto
- ✅ Ordenação cronológica das mensagens
- ✅ Detecção automática de timestamps (milissegundos/segundos)
- ✅ Tratamento de nomes de contatos (usa nome da API quando disponível)
- ✅ Ignora chats sem mensagens

## 🏗️ Arquitetura

O serviço é composto por:

- **UAZAPI Client**: Cliente HTTP para buscar chats e mensagens da API UAZAPI
- **Chatwoot Database**: Acesso direto ao banco PostgreSQL do Chatwoot
- **Sync Service**: Orquestra a sincronização de dados
- **Config**: Gerenciamento de configurações via variáveis de ambiente

## 📦 Pré-requisitos

- Go 1.19 ou superior
- PostgreSQL (banco de dados do Chatwoot)
- Acesso à API UAZAPI
- Token de API do Chatwoot (opcional, para algumas operações)

## 🚀 Instalação

### Instalação Local

1. Clone o repositório:
```bash
git clone https://github.com/adfastltda/uazapi-chatwoot-sync-go.git
cd chatwoot-sync-go
```

2. Instale as dependências:
```bash
go mod download
```

3. Configure as variáveis de ambiente (veja seção [Configuração](#-configuração))

4. Execute o serviço:
```bash
go run main.go
```

Ou usando o Makefile:
```bash
make run
```

### Instalação com Docker

1. Configure as variáveis de ambiente no arquivo `.env` ou `docker-compose.yaml`

2. Execute com Docker Compose:
```bash
docker-compose up -d
```

Ou construa e execute manualmente:
```bash
docker build -t chatwoot-sync .
docker run --env-file .env chatwoot-sync
```

## ⚙️ Configuração

Crie um arquivo `.env` na raiz do projeto com as seguintes variáveis:

### UAZAPI (Obrigatório)

```env
# URL base da API UAZAPI
UAZAPI_BASE_URL=https://free.uazapi.com

# Token de autenticação da API UAZAPI
UAZAPI_TOKEN=seu-token-aqui
```

### Chatwoot Database (Obrigatório)

```env
# Configurações do banco PostgreSQL do Chatwoot
CHATWOOT_DB_HOST=localhost
CHATWOOT_DB_PORT=5432
CHATWOOT_DB_NAME=chatwoot
CHATWOOT_DB_USER=chatwoot
CHATWOOT_DB_PASSWORD=sua-senha-aqui
CHATWOOT_DB_SSLMODE=disable
```

### Chatwoot API (Opcional)

```env
# URL base da API do Chatwoot (para algumas operações)
CHATWOOT_BASE_URL=https://app.chatwoot.com

# Token de API do Chatwoot
CHATWOOT_API_TOKEN=seu-token-aqui
```

### Chatwoot Account/Inbox

```env
# ID da conta no Chatwoot
CHATWOOT_ACCOUNT_ID=1

# ID do inbox (será detectado automaticamente se não especificado)
CHATWOOT_INBOX_ID=1

# Nome do inbox (usado como fallback se INBOX_ID não for encontrado)
CHATWOOT_INBOX_NAME=WhatsApp
```

### Configurações de Sincronização

```env
# Tamanho do lote para processamento (padrão: 1000)
SYNC_BATCH_SIZE=1000

# Limite de chats para sincronizar (padrão: 100000)
SYNC_LIMIT_CHATS=100000

# Limite de mensagens por chat (padrão: 10000)
SYNC_LIMIT_MESSAGES=10000
```

## 📖 Uso

### Execução Básica

```bash
go run main.go
```

### Com Makefile

```bash
# Compilar
make build

# Executar
make run

# Executar com arquivo .env
make run-env

# Limpar artefatos de build
make clean

# Instalar dependências
make install
```

### Com Docker

```bash
# Iniciar serviço
docker-compose up -d

# Ver logs
docker-compose logs -f

# Parar serviço
docker-compose down
```

## 🔄 Fluxo de Sincronização

1. **Busca Chats**: Obtém todos os chats não-grupos da API UAZAPI
2. **Filtra Chats com Mensagens**: Ignora chats que não têm mensagens
3. **Cria/Atualiza Contatos**: Cria ou atualiza contatos no Chatwoot usando o número de telefone
4. **Cria/Atualiza Conversas**: Cria ou atualiza conversas associadas aos contatos
5. **Sincroniza Mensagens**: Para cada chat, busca mensagens e insere apenas as novas
6. **Ordena Mensagens**: Garante que as mensagens sejam inseridas em ordem cronológica
7. **Atualiza Atividade**: Atualiza a última atividade das conversas

## 📁 Estrutura do Projeto

```
chatwoot-sync-go/
├── main.go                 # Ponto de entrada da aplicação
├── go.mod                  # Dependências Go
├── go.sum                  # Checksums das dependências
├── Dockerfile              # Imagem Docker
├── docker-compose.yaml     # Configuração Docker Compose
├── Makefile                # Comandos úteis
├── .gitignore              # Arquivos ignorados pelo Git
└── internal/
    ├── config/             # Configuração e variáveis de ambiente
    │   └── config.go
    ├── models/             # Modelos de dados
    │   ├── models.go       # Modelos UAZAPI e Chatwoot
    │   └── chatwoot.go    # Modelos específicos do Chatwoot
    ├── uazapi/             # Cliente da API UAZAPI
    │   └── client.go
    ├── chatwoot/           # Acesso ao Chatwoot
    │   ├── database.go    # Acesso direto ao banco PostgreSQL
    │   └── api_client.go  # Cliente da API do Chatwoot (opcional)
    └── sync/               # Serviço de sincronização
        └── service.go
```

## 🔧 Desenvolvimento

### Requisitos de Desenvolvimento

- Go 1.19+
- PostgreSQL (para testes locais)
- Acesso à API UAZAPI

### Executar Testes

```bash
make test
```

### Formatar Código

```bash
go fmt ./...
```

### Compilar Binário

```bash
make build
```

O binário será gerado em `bin/chatwoot-sync`.

## 📝 Logs

O serviço gera logs detalhados sobre o processo de sincronização:

- Chats encontrados e processados
- Contatos criados/atualizados
- Conversas criadas/atualizadas
- Mensagens inseridas (por lote)
- Erros e avisos

Exemplo de saída:
```
2025/12/18 00:13:17 Found 14 chats with messages out of 215 total chats
2025/12/18 00:13:18 Created/updated 14 contacts and conversations
2025/12/18 00:13:18 Processing 5 total messages for chat 5521959032485@s.whatsapp.net
2025/12/18 00:13:19 Inserted batch 1-5: 5 messages (total: 5/5) for conversation 1012
```

## ⚠️ Limitações

- Processa apenas mensagens de texto (mídias não são sincronizadas)
- Requer acesso direto ao banco PostgreSQL do Chatwoot
- Processa apenas chats individuais (não grupos)
- Ignora chats sem mensagens

## 🐛 Troubleshooting

### Erro de Conexão com Banco de Dados

Verifique se:
- As credenciais do banco estão corretas
- O PostgreSQL está acessível
- O SSL mode está configurado corretamente

### Erro de Autenticação UAZAPI

Verifique se:
- O token UAZAPI está correto
- A URL base da API está correta
- Há conectividade com a API

### Mensagens Não Aparecem

Verifique se:
- As mensagens não foram inseridas anteriormente (o serviço ignora duplicatas)
- Os timestamps estão corretos
- O `source_id` está no formato correto (`WAID:{message_id}`)

## 📄 Licença

Sla, deixando os creditos ja ta bom.

## 🤝 Contribuindo

Contribuições são bem-vindas! Por favor:

1. Faça um fork do projeto
2. Crie uma branch para sua feature (`git checkout -b feature/AmazingFeature`)
3. Commit suas mudanças (`git commit -m 'Add some AmazingFeature'`)
4. Push para a branch (`git push origin feature/AmazingFeature`)
5. Abra um Pull Request

## 📞 Suporte

Para suporte, abra uma issue no repositório ou entre em contato com a equipe de desenvolvimento.

---

**Desenvolvido com ❤️ usando Go**

