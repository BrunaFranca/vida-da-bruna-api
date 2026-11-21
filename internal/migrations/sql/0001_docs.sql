-- Banco de documentos: espelha a interface Store do app (coleção + id + dados livres em JSON).
CREATE TABLE docs (
    colecao       text        NOT NULL,
    id            text        NOT NULL,
    dados         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (colecao, id)
);
