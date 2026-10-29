# Roteiro de Demonstração Acadêmica com Wireshark

> **Projeto:** NetProbe Wi-Fi — Mapeador de rede Wi-Fi local em tempo real  
> **Público-alvo:** Professor e banca avaliadora  
> **Objetivo:** Demonstrar a observabilidade de rede dos **5 elementos obrigatórios** inspecionando pacotes reais capturados no Wireshark.

---

## 1. Preparação da Demonstração

1. **Abra o Wireshark** na máquina de apresentação.
2. **Selecione a interface Wi-Fi ativa** (ex.: `wlp2s0`, `wlan0` no Linux ou `Wi-Fi` no Windows).
3. **Inicie a captura de pacotes** no Wireshark (ícone de barbatana azul).
4. **Abra o NetProbe Wi-Fi**.
5. Verifique se a interface selecionada no aplicativo corresponde à interface sendo capturada no Wireshark.

---

## 2. Roteiro Passo a Passo pelos 5 Elementos

### Elemento 1 — Varredura ICMP (Ping Sweep com Rate Limit e RTT)

#### O que demonstrar:
O envio controlado de pacotes ICMP Echo Request para a sub-rede selecionada, medição precisa de RTT (*Round Trip Time*) e respeito estrito ao limite de pacotes por segundo (PPS).

#### Filtro de Exibição no Wireshark:
```text
icmp.type == 8 || icmp.type == 0
```

#### Passos na UI:
1. Configure a faixa (ex.: `192.168.1.0/24`).
2. Defina a taxa para `50` ou `100` pacotes/s e timeout de `1000 ms`.
3. Clique em **Iniciar Varredura**.

#### O que explicar e apontar no Wireshark:
* **Echo Request (`icmp.type == 8`):** Aponte os pacotes saindo sequencialmente no ritmo exato do *rate limiter* configurado (sem sobrecarregar o ponto de acesso Wi-Fi).
* **Echo Reply (`icmp.type == 0`):** Mostre as respostas dos dispositivos ativos (roteador, celulares, laptops).
* **Cálculo de RTT:** Selecione um pacote `Echo Reply`; no painel inferior do Wireshark, aponte o campo `[Response time: X.XX ms]`. Mostre que o valor bate com a coluna **RTT** na tabela do NetProbe Wi-Fi.

---

### Elemento 2 — Descoberta via ARP Provocado (Hosts Silenciosos/Firewall)

#### O que demonstrar:
Como o NetProbe descobre dispositivos que **bloqueiam ping ICMP** (como máquinas Windows com o *Windows Defender Firewall* ativo) sem necessitar de privilégios de superusuário (`root`/`Admin`) e sem usar drivers externos como Npcap/libpcap.

#### Filtro de Exibição no Wireshark:
```text
arp || (udp.dstport == 38472)
```

#### O que explicar e apontar no Wireshark:
1. **O Gatilho UDP Unicast:**
   * Explique: *"Para os IPs que não responderam ao ICMP Echo no Elemento 1, o NetProbe dispara um datagrama UDP unicast para uma porta alta fechada (`38472`)"*.
   * Mostre o pacote UDP no Wireshark.
2. **A Reação Automática do Kernel:**
   * Aponte no Wireshark: Imediatamente após o envio do UDP, o próprio kernel do sistema operacional (Linux ou Windows) emite automaticamente na camada de enlace (L2):
     ```text
     ARP Request: Who has 192.168.1.X? Tell 192.168.1.Y
     ```
   * O host silencioso responde com:
     ```text
     ARP Reply: 192.168.1.X is at [Endereço MAC]
     ```
3. **Resultado na UI:**
   * Mostre que o host aparece na tabela com a tag visual âmbar **`ARP (Silencioso)`** ou **`Firewall`**, evidenciando que foi descoberto na camada 2 mesmo sem responder ao ICMP da camada 3.

---

### Elemento 3 — Vínculo IP ↔ MAC e Resolução de Fabricante (IEEE OUI)

#### O que demonstrar:
A associação unívoca entre o endereço IP e o endereço físico (MAC), descartando entradas incompletas (`0x0`), identificando MACs privados/aleatórios e resolvendo o fabricante do hardware via base binária IEEE embutida.

#### Filtro de Exibição no Wireshark:
```text
arp.opcode == 2
```
*(Filtra apenas respostas `ARP Reply`)*

#### O que explicar e apontar no Wireshark:
1. Selecione um pacote `ARP Reply` no Wireshark.
2. No painel de detalhes do pacote, expanda `Address Resolution Protocol (reply)` e aponte para o campo `Sender MAC address` (ex.: `38:97:d6:xx:xx:xx`).
3. Destaque os primeiros 3 bytes (`38:97:d6`): este é o **OUI** (*Organizationally Unique Identifier*).
4. Aponte para a linha correspondente na tabela da UI do NetProbe:
   * Mostre que o fabricante identificado (ex.: `ZTE Corporation`, `Apple`, `Intel`) é idêntico ao registrado no Wireshark.
   * Explique que a base OUI da IEEE está **comprimida e embutida no binário** com busca binária $\mathcal{O}(\log N)$, operando offline sem fazer nenhuma requisição HTTP externa.
5. Se algum smartphone na rede usar MAC aleatório (recurso de privacidade moderno), mostre que o NetProbe detecta o bit localmente administrado e rotula como **`Privado / Aleatório`**.

---

### Elemento 4 — Interface em Tempo Real e Agrupamento em Lotes

#### O que demonstrar:
A reatividade da aplicação durante a varredura, com contadores atômicos, barra de progresso fluida e atualização em lotes (*batching*) para não sobrecarregar a renderização da interface.

#### Passos na UI:
1. Inicie a varredura e observe os componentes:
   * **Contadores de Métricas:** Ativos Totais, Hosts ICMP, Hosts ARP/Firewall e RTT Médio atualizando em tempo real.
   * **Barra de Progresso:** Atualização percentual precisa da sub-rede.
   * **Tabela de Hosts:** Novos hosts entrando em lotes suaves (agrupamento a cada ~100 ms via `EventAggregator` no Go).
2. Demonstre o botão de **Cancelar Varredura**:
   * Clique em Cancelar durante a varredura.
   * No Wireshark, aponte que os pacotes param de ser emitidos instantaneamente (graças ao cancelamento gracioso via `context.Context` do Go).

---

### Elemento 5 — Varredura de Portas TCP (TCP Connect Sweep)

#### O que demonstrar:
Varredura de portas TCP selecionadas com medição de latência, controle estrito de concorrência global (semáforo de 200 conexões máximas) e fechamento ordenado das conexões.

#### Filtro de Exibição no Wireshark:
```text
tcp.flags.syn == 1
```

#### Passos na UI:
1. Na tabela de hosts, clique no botão **Portas** de um host ativo (ex.: o roteador `192.168.1.1`).
2. O painel lateral (*slide-over drawer*) se abrirá à direita.
3. Selecione as portas de teste (ex.: portas `20` a `1000` ou `80, 443, 8080`) e clique em **Escanear Portas**.

#### O que explicar e apontar no Wireshark:
1. **Three-Way Handshake TCP:**
   * **Porta Aberta:** Mostre o pacote `[SYN]` enviado pelo NetProbe, o `[SYN, ACK]` retornado pelo alvo e o `[ACK]` ou `[RST]` de encerramento da conexão.
   * **Porta Fechada:** Mostre o pacote `[SYN]` enviado e a resposta imediata `[RST, ACK]` do alvo.
2. **Concorrência Controlada:**
   * Explique que o motor utiliza um canal bufferizado em Go como semáforo estrito (`chan struct{}` com limite de 200 conexões), evitando esgotar descritores de arquivo (`ulimit -n`) e sem acionar proteções contra flood no roteador.
3. **Resultado na UI:**
   * Mostre as portas abertas listadas no drawer lateral com seu respectivo serviço (ex.: `80/HTTP`, `443/HTTPS`, `53/DNS`, `22/SSH`) e tempo de resposta.

---

## 3. Resumo dos Filtros do Wireshark para Consulta Rápida

| Objetivo | Filtro Wireshark |
|---|---|
| **Elemento 1 (ICMP Sweep)** | `icmp.type == 8 \|\| icmp.type == 0` |
| **Elemento 2 (ARP Provocado)** | `arp \|\| (udp.dstport == 38472)` |
| **Elemento 3 (Respostas ARP / MAC)** | `arp.opcode == 2` |
| **Elemento 5 (Portas TCP / Handshake)** | `tcp.flags.syn == 1` |
| **Visão Geral Completa do NetProbe** | `icmp \|\| arp \|\| udp.dstport == 38472 \|\| (tcp.flags.syn == 1)` |

---

## 4. Destaques Conceituais para Apresentar à Banca

1. **Arquitetura Zero-Driver:** Não usamos CGO no scanner e não exigimos a instalação de drivers externos invasivos (como Npcap ou WinPcap). Tudo é implementado sobre as APIs e sockets padrão do sistema operacional.
2. **Segurança por Padrão:** O backend rejeita ativamente varreduras fora das faixas privadas (RFC 1918 / Link-Local) e restringe o CIDR a `/22` (máximo de 1.022 hosts), impedindo abusos ou ataques a redes externas.
3. **Alta Performance Offline:** A base de fabricantes IEEE com mais de 50.000 registros ocupa menos de 1 MB de memória RAM e executa em microssegundos via busca binária.
