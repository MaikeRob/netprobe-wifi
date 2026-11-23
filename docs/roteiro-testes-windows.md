# Roteiro de Testes Manuais em Ambiente Windows

> **Projeto:** NetProbe Wi-Fi  
> **Sistemas Alvo:** Windows 10 e Windows 11 (64-bit AMD64)  
> **Objetivo:** Roteiro de validação manual das funcionalidades no Windows, confirmando conformidade com as regras de privilégios mínimos e compatibilidade de rede.

---

## 1. Pré-Requisitos e Ambiente

1. **Sistema Operacional:** Windows 10 (versão 1809+) ou Windows 11.
2. **WebView2 Runtime:**
   * No Windows 11 e versões modernas do Windows 10, o Microsoft Edge WebView2 Runtime já vem instalado de fábrica.
   * Se ausente, o instalador NSIS gerado pelo Wails aciona automaticamente o instalador *Evergreen Bootstrapper* da Microsoft.
3. **Privilégios:**
   * O aplicativo foi projetado para rodar como **usuário comum**.
   * **Não execute como Administrador** e observe que nenhum prompt do UAC (*Controle de Conta de Usuário*) deve ser solicitado.

---

## 2. Instalação e Execução

### Opção A: Executável Avulso
1. Copie o binário `netprobe-wifi.exe` gerado em `build/bin/` para o computador com Windows.
2. Dê um duplo clique no arquivo `netprobe-wifi.exe`.
3. Observe que o aplicativo abre diretamente a janela frameless com design Organic Modernism.

### Opção B: Instalador NSIS
1. Execute o instalador `netprobe-wifi-amd64-installer.exe`.
2. Siga as etapas normais do assistente de instalação.
3. Abra o NetProbe Wi-Fi pelo menu Iniciar ou atalho na Área de Trabalho.

---

## 3. Matriz de Verificação por Elemento

| Etapa | Teste | Comportamento Esperado | Status |
|---|---|---|---|
| **1. Interfaces** | Abertura do app | A interface Wi-Fi é detectada automaticamente com IP, MAC e CIDR `/24` sugerido (ex.: `192.168.1.0/24`) | [ ] |
| **2. ICMP (Elem. 1)** | Iniciar varredura `/24` | Os pings são disparados via `IcmpSendEcho` (`iphlpapi.dll`). Hosts que aceitam ping respondem com RTT medido e badge `Ativo via ICMP`. | [ ] |
| **3. ARP (Elem. 2)** | Varredura de hosts silenciosos | Dispositivos que bloqueiam ping são sondados via `SendARP` (`iphlpapi.dll`) e descobertos na tabela de vizinhos (`GetIpNetTable`) com badge `Descoberto via ARP`. | [ ] |
| **4. OUI (Elem. 3)** | Resolução de fabricante | MACs válidos exibem fabricante real; smartphones com MAC aleatório exibem badge `MAC Randômico / Privado`. | [ ] |
| **5. UI (Elem. 4)** | Reatividade e contadores | Barra de progresso atualiza continuamente; cancelamento interrompe a varredura imediatamente. | [ ] |
| **6. Portas (Elem. 5)** | Selecionar host e escanear portas | Drawer lateral abre na aba `Abertas`; portas abertas e serviços são listados no topo com tempos de resposta. | [ ] |

---

## 4. Teste de Validação do Elemento 2 com o Windows Defender Firewall

O Windows é o ambiente ideal para comprovar a eficácia do Elemento 2 (ARP Provocado):
1. Por padrão, o **Windows Defender Firewall** bloqueia pacotes ICMP Echo de entrada no perfil de rede pública.
2. Em uma máquina Windows alvo com firewall ativado:
   * O NetProbe envia ICMP Echo Request -> o alvo descarta silenciosamente.
   * No Estágio 2, o NetProbe aciona `SendARP` (`iphlpapi.dll`) no Windows (ou o pacote UDP na porta `38472` quando executado a partir do Linux) -> a pilha de rede do alvo responde obrigatoriamente ao `ARP Request` com seu endereço MAC.
   * O host aparece na tabela do NetProbe rotulado com a badge âmbar **`Descoberto via ARP`** (e permanece sendo detectado mesmo em varreduras repetidas consecutivas).
3. Isso comprova para a banca que o NetProbe mapeia hosts reais mesmo quando ferramentas tradicionais como o `ping` reportam que o host está offline.

---

## 5. Verificação de Segurança e Ausência de Npcap

* Abra o **Painel de Controle -> Programas e Recursos**.
* Certifique-se de que o **Npcap** ou **WinPcap** **NÃO** estão instalados.
* O NetProbe Wi-Fi continuará funcionando normalmente, provando que sua implementação é 100% nativa via APIs oficiais do Windows (`iphlpapi.dll` e `ws2_32.dll`).
