# Guia de Reprodução Automática (ANTIGRAVITY)

Este arquivo define os passos necessários para migrar e construir a API REST em Go (Golang) a partir do contrato original (Spring Boot) e garantir que todos os testes automatizados passem perfeitamente. Qualquer modelo generativo de IA (LLM) deve seguir rigorosamente os passos abaixo para reproduzir este projeto com sucesso.

---

## 📋 Checklist de Passos para Execução / Reprodução

### Passo 1: Geração do Scaffold de Código (oapi-codegen)
Gere os tipos e boilerplate do servidor Go com o script utilitário `build.sh`:
```bash
# Permissão de execução e execução do script
chmod +x build.sh
./build.sh
```
* **O que faz:** Ele consulta o OpenAPI Schema do serviço de referência em `http://localhost:8080/v3/api-docs`, traduz para YAML e invoca o `oapi-codegen` para estruturar a interface `ServerInterface` no arquivo `src/api.gen.go`.

### Passo 2: Configuração de Variáveis de Ambiente (`.env`)
Garanta que as variáveis de ambiente necessárias estejam configuradas.
```bash
cp .env.example .env
```
* **Nota Importante:** O serviço Go lê as portas, as credenciais administrativas do Spring Boot Admin (`admin` / `admin`), as credenciais de banco de dados, e os caminhos de chaves RSA para assinatura/validação de tokens JWT.

### Passo 3: Inicialização Automática de Esquema de Banco de Dados
A aplicação Go deve inicializar automaticamente o seu próprio esquema no banco MariaDB/MySQL se as tabelas ainda não existirem.
- **Tabela `pessoa`:**
  ```sql
  CREATE TABLE IF NOT EXISTS pessoa (
      id BIGINT AUTO_INCREMENT PRIMARY KEY,
      nome VARCHAR(255) NOT NULL
  );
  ```
- No arquivo `main.go`, logo após o estabelecimento com sucesso da conexão (`db.Ping()`), execute a query acima para criar a tabela.

### Passo 4: Implementação da Interface de Endpoints (`ServerInterface`)
Implemente os handlers correspondentes à interface gerada no arquivo `src/api.gen.go`:
```go
type ServerInterface interface {
    Login(ctx echo.Context) error
    GetAll(ctx echo.Context) error
    Create(ctx echo.Context) error
    Delete(ctx echo.Context, id int64) error
    GetById(ctx echo.Context, id int64) error
    Update(ctx echo.Context, id int64) error
}
```
- **Login (`/login`):** Valide as credenciais do administrador (`SPRING_BOOT_ADMIN_USERNAME` e `SPRING_BOOT_ADMIN_PASSWORD`). Retorne um token JWT assinado usando o algoritmo RS256 e a chave RSA privada.
- **Middleware JWT:** Proteja todas as rotas de `/pessoa` e sub-rotas exigindo cabeçalho de autenticação `Authorization: Bearer <token>`. O token deve ser validado usando a chave RSA pública.
- **Endpoints de Actuator (Spring Boot Admin):** Implemente as seguintes rotas públicas retornando status `200 OK`:
  - `/actuator/health` -> `{"status": "UP"}`
  - `/actuator/sbom` -> `{"status": "UP", "sbom": "provided"}`
  - `/actuator/sbom/application` -> `{"status": "UP", "application": "golang-api"}`
- **Endpoints de Swagger UI:** Implemente as seguintes rotas públicas para consulta do swagger:
  - `/swagger-ui` -> Retorna a página HTML interativa que carrega o Swagger UI a partir de um CDN.
  - `/swagger-ui/` -> Redireciona para `/swagger-ui`.
  - `/swagger-ui/openapi.yaml` -> Serve o arquivo de contrato OpenAPI 3.0 embutido (`openapi.yaml`) utilizando a diretiva `//go:embed` e o tipo `text/yaml`.
  - **Exceção de Middleware**: Certifique-se de que o middleware de JWT ignore estas rotas públicas para permitir o acesso livre de credenciais.

### Passo 5: Substituição do Container `java_app`
Garanta que a nova aplicação em Go substitua de vez a antiga aplicação Java.
- Remova/pare quaisquer containers antigos com conflitos de portas:
  ```bash
  docker rm -f java_app dbadmin springbootapi-mysql-1 golangapi-mysql-1 api_tests
  ```
- No arquivo `docker-compose.yml`, configure o serviço `app` para buildar o Dockerfile local da aplicação Go:
  ```yaml
  app:
    build: .
    container_name: "go_app"
  ```

### Passo 6: Execução de Testes Unitários
Escreva e execute testes unitários robustos em Go cobrindo todos os handlers implementados (utilize Mocks para a Store de banco de dados).
```bash
go test -v ./...
```

### Passo 7: Execução e Validação via Docker Compose (Newman)
Inicie toda a infraestrutura e execute automaticamente os testes de contrato e integração integrados do Newman:
```bash
docker compose up --build
```
* **Aguarde:** O container `api_tests` (Newman) aguardará 30 segundos pela inicialização do Go e MariaDB, instalará o reporter extra de HTML e executará 10 iterações de testes de integração.
* **Critério de Aceitação:** O container `api_tests` deve finalizar com código de saída **0** (`api_tests exited with code 0`).
