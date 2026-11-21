-- Tempo real: a cada mudança em docs, o Postgres avisa no canal "docs" quem mudou.
-- Todas as réplicas da API escutam (LISTEN) e repassam aos clientes conectados por SSE.
CREATE FUNCTION docs_notificar() RETURNS trigger AS $$
DECLARE
    linha docs;
BEGIN
    IF TG_OP = 'DELETE' THEN
        linha := OLD;
    ELSE
        linha := NEW;
    END IF;
    PERFORM pg_notify('docs', json_build_object('colecao', linha.colecao, 'id', linha.id)::text);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER docs_mudou
    AFTER INSERT OR UPDATE OR DELETE ON docs
    FOR EACH ROW EXECUTE FUNCTION docs_notificar();
