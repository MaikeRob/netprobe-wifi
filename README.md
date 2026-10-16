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
├── docs/
│   └── adr/                 # Registro de Decisões de Arquitetura (ADRs)
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

## Decisões de Arquitetura (ADRs)

- [ADR 0001: Arquitetura de Camadas e Stack Tecnológica](docs/adr/0001-arquitetura-e-stack.md)
- [ADR 0002: Base OUI IEEE Embutida e Resolução de Fabricantes](docs/adr/0002-base-oui.md)
- [ADR 0003: Política de Alvos, Descoberta de Interface e Ausência de Tráfego](docs/adr/0003-politica-de-alvos-e-interface-padrao.md)
- [ADR 0004: Mecanismo de Varredura ICMP, Multiplexação e Privilégios](docs/adr/0004-mecanismo-icmp-e-privilegios.md)
- [ADR 0005: ARP Provocado pelo Kernel, Resolução de Vizinhos e Vínculo IP ↔ MAC](docs/adr/0005-arp-provocado-e-tabela-vizinhos.md)
- [ADR 0006: Varredura de Portas TCP, Connect Scan e Controle de Concorrência](docs/adr/0006-varredura-portas-tcp.md)
- [ADR 0007: Orquestração de Varredura, Lotes de Eventos e Wails Backend](docs/adr/0007-orquestracao-lotes-e-wails-backend.md)
- [ADR 0008: Interface Gráfica em Tempo Real e Estética Minimalista Organic Modernism](docs/adr/0008-interface-grafica-e-estetica-minimalista.md)

---

## Roteiro e Fases do Projeto

O detalhamento e status das etapas planejadas de implementação estão consolidados em:
- [Checklist de Fases do Projeto](docs/checklist-fases.md)

---

## Aviso de Uso Ético e Autorizado

Este software foi concebido estritamente para finalidades acadêmicas e diagnósticas em ambientes de rede sob controle do usuário ou com consentimento explícito. O backend implementa proteções nativas que bloqueiam varreduras acidentais contra redes públicas ou contra o próprio host (loopback/broadcast).
