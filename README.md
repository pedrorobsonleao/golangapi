# API Golang - Guia de Construção e Estrutura

Este repositório contém a base e os recursos necessários para a migração/construção de uma API REST desenvolvida em **Go (Golang)**, baseada em um contrato de referência pré-existente (Spring Boot) e validada via testes de integração automatizados (Newman/Postman).

---

## 1. Estrutura do Projeto

Abaixo está o detalhamento dos componentes fornecidos no projeto:

*   **`src/`**: Diretório reservado para o código-fonte em Go. Atualmente está vazio e será onde os handlers, middlewares, modelos de banco de dados e rotas serão organizados.
*   **`db/`**: Armazena as configurações e segredos do banco de dados relacional. Contém o arquivo `pwd.txt` com a senha do root.
*   **`newman/`**: Contém a suite de testes de integração e relatórios de execução:
    *   `tests/simple_spring_boot_rest_api.postman_collection.json`: Coleção do Postman com todas as requisições de teste que definem o comportamento esperado da API (como tratamento de sucesso, erros de validação, autenticação, etc.).
    *   `tests/test-local.postman_environment.json` & `test-docker.postman_environment.json`: Variáveis de ambiente utilizadas pelos testes locais ou em containers.
    *   `tests/newman/`: Relatórios gerados em HTML contendo os resultados de execuções anteriores dos testes.
*   **`docker-compose.yml`**: Orquestra os serviços do ecossistema:
    *   `mysql` (MariaDB): Instância de banco de dados que persiste as informações da API.
    *   `app` (Referência Java Spring Boot): A aplicação de referência. Seu contrato OpenAPI é lido para gerar os scaffolds em Go.
    *   `phpadmin` (phpMyAdmin): Interface web para administração do banco de dados MySQL, disponível por padrão na porta `9090`.
    *   `newman`: Container que executa testes automatizados contra a API a cada inicialização para validar o contrato e a lógica de negócio.
*   **`build.sh`**: Script Bash utilitário que:
    1. Executa um `curl` contra a API de referência Java (`http://localhost:8080/v3/api-docs`) para obter as especificações em formato JSON.
    2. Converte o formato JSON para YAML utilizando o comando `yq`.
    3. Executa o gerador `oapi-codegen` para ler as especificações OpenAPI e gerar o boilerplate do servidor Go (`types` e `server`) salvando-o no arquivo `src/api.gen.go`.
*   **`.env` & `.env.example`**: Configurações de credenciais de banco de dados, portas de rede e chaves privadas/públicas.

---

## 2. Como Construir a API Golang Utilizando os Recursos Fornecidos

O desenvolvimento da API em Go segue um modelo guiado por contrato (Contract-First / API-First). A construção deve seguir as seguintes etapas:

### Passo A: Geração de Código a Partir do OpenAPI
Com a aplicação Java de referência rodando em background (portas mapeadas no docker-compose):
```bash
# Executa o script de codegen para gerar o scaffold em src/api.gen.go
chmod +x build.sh
./build.sh
```
Isso criará a estrutura base dos endpoints, structs de requisição e resposta do OpenAPI de forma automatizada no pacote `api`.

### Passo B: Setup do Ambiente Go
Inicialize e organize as dependências Go necessárias:
1. O módulo Go já foi inicializado:
   ```bash
   go mod init github.com/pedrorobsonleao/golangapi
   ```
2. Instale o roteador HTTP que decidir utilizar (ex: Echo, Chi ou Gin) e o runtime do `oapi-codegen`:
   ```bash
   go get github.com/oapi-codegen/runtime
   # Caso use o roteador padrão (Echo no oapi-codegen):
   go get github.com/labstack/echo/v4
   ```

### Passo C: Implementação dos Endpoints (Handlers)
Crie o arquivo principal (ex: `src/main.go` ou crie subpacotes dentro de `src/`) para:
1. **Configuração e Conexão de Banco**:
   * Ler variáveis de ambiente do arquivo `.env` (usando libs como `github.com/joho/godotenv`).
   * Abrir conexão com o MariaDB/MySQL (usando o driver `github.com/go-sql-driver/mysql` ou `gorm.io/gorm`).
2. **Implementar a Interface**:
   * O `api.gen.go` exporta uma interface de servidor (ex: `ServerInterface`). Crie uma struct Go (ex: `Server struct {}`) e implemente todos os métodos exigidos por esta interface (CRUD de pessoas, Login, etc.).
3. **Mecanismo de Autenticação / Segurança**:
   * A coleção do Postman define testes para o endpoint de `/login` e rotas protegidas. Certifique-se de implementar a validação de tokens JWT gerados por chaves RSA, utilizando os caminhos configurados nas variáveis `RSA_PRIVATE_KEY_PATH` e `RSA_PUBLIC_KEY_PATH`.

### Passo D: Substituição no Docker Compose
Depois que a API Go estiver funcional, crie um `Dockerfile` para a aplicação Go e atualize o serviço `app` no arquivo [docker-compose.yml](file:///home/pleao/Documents/Projects/golang/golangapi/docker-compose.yml):
```yaml
  app:
    build: 
      context: .
      dockerfile: Dockerfile  # Dockerfile da API Go
    container_name: "go_app"
```

### Passo E: Execução dos Testes com Newman
Para testar a aderência e corretude da sua API Go frente aos requisitos de integração:
```bash
docker compose up --build
```
Isso iniciará o banco de dados MySQL, o phpMyAdmin, a sua aplicação Go e, por fim, executará o container `newman` que disparará os testes contra a API em Go. Verifique os relatórios gerados em `newman/tests/newman/` para assegurar que todos os testes passaram com sucesso (status `200 OK`, validações de inputs longos/curtos, `403 Forbidden`, etc.).
