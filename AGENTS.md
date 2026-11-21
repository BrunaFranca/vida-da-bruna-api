# AGENTS.md

Contexto para assistentes de IA neste repositório.

- **O que é:** a API em Go do app pessoal Vida da Bruna, e ao mesmo tempo um lab público de SRE da Bruna (DevOps/SRE). O app em si fica num repo privado separado (`vida-da-bruna`).
- **Repo público:** nunca colocar dados pessoais ou financeiros, nem código/config de empregador.
- **Plano e etapas:** `docs/adr/0001-lab-sre-api-go.md`. Atualizar o status lá ao concluir cada etapa.
- **Idioma:** comentários, commits e docs em português. Conventional Commits.
- **Antes de commit:** `go vet ./...` e `go test ./...` precisam passar.
- **Rodar local:** `docker compose up --build` → http://localhost:8080/readyz
- No Windows da Bruna: Go em `C:\Program Files\Go\bin`, Git em `C:\Program Files\Git\cmd`, gh em `C:\Program Files\GitHub CLI`.
