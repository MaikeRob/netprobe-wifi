# netprobe-wifi

Aplicativo desktop em tempo real para mapeamento e diagnóstico de redes locais Wi-Fi (IPv4), projetado para demonstração acadêmica e observabilidade transparente via Wireshark.

## Funcionalidades Principais (Os 5 Elementos)
1. **Varredura ICMP (Ping Sweep):** Verificação de hosts ativos com medição de RTT e timeout parametrizável.
2. **Descoberta via ARP provocado:** Detecção de hosts que silenciam ICMP (firewall ativo).
3. **Vínculo IP ↔ MAC e Fabricante:** Normalização de MACs, identificação de fabricantes com base IEEE OUI embutida e detecção de MACs randômicos/privados.
4. **Interface em Tempo Real:** Painel com contadores consolidados, progresso e tabela de hosts com atualizações agrupadas em lotes.
5. **Varredura de Portas TCP:** Connect scan em intervalo parametrizável com limitação global de descritores de conexões.

---

## Estrutura do Repositório

```text
netprobe-wifi/
├── docs/                    # Roteiros de testes e guias de demonstração
├── frontend/                # React 19 + TypeScript + Tailwind CSS (Organic Modernism)
│   ├── src/                 # Componentes, estado e integração Wails
│   └── wailsjs/             # Bindings gerados automaticamente pelo Wails CLI
├── internal/
│   ├── netinfo/             # Descoberta de interfaces, rota padrão e política de alvos
│   ├── oui/                 # Base OUI da IEEE embutida e utilitários MAC
│   │   └── gen/             # Ferramenta para conversão do CSV oficial da IEEE
│   └── scanner/
│       ├── icmp/            # Motor de varredura ICMP (ping sweep) multiplataforma
│       ├── arp/             # ARP provocado pelo kernel e vínculo IP ↔ MAC
│       ├── port/            # Varredura de portas TCP (connect scan) com controle de concorrência
│       ├── aggregator.go    # Agregador de eventos em lotes (~100 ms)
│       ├── events.go        # Contratos de dados de eventos tipados
│       └── orchestrator.go  # Orquestrador unificado dos 5 elementos de varredura
├── app.go                   # Camada fina de métodos expostos ao Wails
├── main.go                  # Ponto de entrada da aplicação desktop Wails v2
├── wails.json               # Configuração do projeto Wails
├── go.mod                   # Módulo Go (netprobe-wifi)
└── README.md
```

---

## Pré-requisitos de Desenvolvimento

- **Go:** versão estável `1.27.x`
- **Node.js:** versão LTS `v24.x`
- **Wails:** CLI `v2.16.0` (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`)
- **Linux (Ubuntu 24.04 / Zorin OS 18 ou derivados):**
  - `build-essential`, `pkg-config`, `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`
  - Tag obrigatória de compilação: `-tags webkit2_41` (ex.: `wails dev -tags webkit2_41` ou `go build -tags webkit2_41 .`)

---

## Verificação e Testes

Para executar a suíte completa de testes automatizados com detector de condições de corrida (*data race detector*):

```bash
# Formatação
gofmt -l .

# Análise estática do Go
go vet -tags webkit2_41 ./...

# Testes unitários com detector de corrida
go test -tags webkit2_41 -v -race -count=1 ./...

# Validação de compilação cruzada para Windows (sem CGO)
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go build -o /dev/null .

# Validação de compilação nativa para Linux
go build -tags webkit2_41 .

# Testes e validação do Frontend
cd frontend
npm run typecheck
npm test
npm run build
```

---


## Empacotamento e Distribuição

### Linux (Pacote `.deb` e Binário Nativo)
Para gerar o pacote de instalação `.deb` com aplicação automática da permissão `CAP_NET_RAW` no binário:
```bash
chmod +x build/linux/build-deb.sh
./build/linux/build-deb.sh
```
O pacote `.deb` e o binário serão gerados em `build/bin/`.

### Windows (Instalador NSIS e Executável)
Para gerar o instalador do Windows com download automático do WebView2 Runtime:
```bash
chmod +x build/windows/build-windows.sh
./build/windows/build-windows.sh
# Ou diretamente via Wails CLI:
# wails build -platform windows/amd64 -nsis -webview2 download
```

---

## Roteiros e Guias de Demonstração

- [Roteiro de Demonstração Acadêmica no Wireshark](docs/roteiro-wireshark.md) — Filtros de captura e explicação técnica passo a passo dos 5 elementos.
- [Roteiro de Testes Manuais em Ambiente Windows](docs/roteiro-testes-windows.md) — Validação sem privilégios de Administrador e teste com o Windows Defender Firewall.
- [Checklist de Fases do Projeto](docs/checklist-fases.md) — Rastreamento de entregas de todas as fases (1 a 7).

---

## Aviso de Uso Ético e Autorizado

Este software foi concebido estritamente para finalidades acadêmicas e diagnósticas em ambientes de rede sob controle do usuário ou com consentimento explícito. O backend implementa proteções nativas que bloqueiam varreduras acidentais contra redes públicas ou contra o próprio host (loopback/broadcast).

