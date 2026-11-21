# AGENTS.md

Contexto para assistentes de IA neste repositório.

- **O que é:** a API em Go do app pessoal Vida da Bruna, e ao mesmo tempo um lab público de SRE da Bruna (DevOps/SRE). O app em si fica num repo privado separado (`vida-da-bruna`).
- **Repo público:** nunca colocar dados pessoais ou financeiros, nem código/config de empregador.
- **Plano e etapas:** `docs/adr/0001-lab-sre-api-go.md`. Atualizar o status lá ao concluir cada etapa.
- **Idioma:** comentários, commits e docs em português. Conventional Commits.
- **Antes de commit:** `go vet ./...` e os testes precisam passar. No Windows dela o Smart App Control bloqueia o binário de teste; rodar dentro do container (com o `db` do Compose no ar):
  `docker run --rm --network vida-da-bruna-api_default -v "${PWD}:/src" -v gomod:/go/pkg/mod -w /src -e TEST_DATABASE_URL="postgres://vida:vida-local@db:5432/vida?sslmode=disable" golang:1.27 go test -race ./...`
- **Rotas:** `GET/POST /v1/{col}`, `GET/PUT/PATCH/DELETE /v1/{col}/{id}` (PUT substitui, PATCH mescla raso, DELETE é idempotente). Migrations em `internal/migrations/sql/`, aplicadas na subida.
- **Rodar local:** `docker compose up --build` → http://localhost:8080/readyz
- No Windows da Bruna: Go em `C:\Program Files\Go\bin`, Git em `C:\Program Files\Git\cmd`, gh em `C:\Program Files\GitHub CLI`.
