// Tráfego "normal" para a API: cada iteração cria, lê, edita e apaga um documento.
// Uso: docker compose --profile carga run --rm k6
//      DURACAO=30m docker compose --profile carga run --rm -e DURACAO k6
import http from "k6/http";
import { check } from "k6";

const ALVO = __ENV.ALVO || "http://localhost:8080";
const JSON_ = { headers: { "Content-Type": "application/json" } };

export const options = {
  scenarios: {
    normal: {
      executor: "constant-arrival-rate",
      rate: 2, // iterações por segundo (≈ 9 requisições/s)
      timeUnit: "1s",
      duration: __ENV.DURACAO || "15m",
      preAllocatedVUs: 5,
      maxVUs: 30,
    },
  },
  // Durante o incidente simulado os erros são esperados: o k6 não deve abortar.
  thresholds: {},
};

export default function () {
  const criado = http.post(`${ALVO}/v1/carga`, JSON.stringify({ titulo: "tarefa de teste", feito: false }), JSON_);
  if (!check(criado, { "criou (201)": (r) => r.status === 201 })) return;
  const id = criado.json("id");

  check(http.get(`${ALVO}/v1/carga/${id}`), { "leu (200)": (r) => r.status === 200 });
  check(http.patch(`${ALVO}/v1/carga/${id}`, JSON.stringify({ feito: true }), JSON_), { "editou (204)": (r) => r.status === 204 });
  check(http.del(`${ALVO}/v1/carga/${id}`), { "apagou (204)": (r) => r.status === 204 });

  // De vez em quando, lista uma coleção pequena.
  if (Math.random() < 0.25) check(http.get(`${ALVO}/v1/config`), { "listou (200)": (r) => r.status === 200 });
}
