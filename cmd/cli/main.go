package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"netprobe-wifi/internal/netinfo"
	"netprobe-wifi/internal/oui"
	"netprobe-wifi/internal/scanner"
	"netprobe-wifi/internal/scanner/arp"
	"netprobe-wifi/internal/scanner/icmp"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("       NetProbe Wi-Fi — Backend CLI Tester        ")
	fmt.Println("==================================================")

	// 1. Diagnostic capabilities
	caps := icmp.CheckCapabilities()
	if caps.CanSendICMP {
		fmt.Printf("[OK] ICMP: Ativo (%s)\n", caps.Reason)
	} else {
		fmt.Printf("[WARN] ICMP: Restrito (%s)\n", caps.Reason)
		fmt.Println("       -> O backend executará em modo resiliente via ARP provocado.")
		fmt.Println("       -> Para ativar ICMP: sudo sysctl -w net.ipv4.ping_group_range=\"0 2147483647\"")
	}

	// 2. Discover interfaces
	ifaces, err := netinfo.ListInterfaces()
	if err != nil {
		log.Fatalf("Erro listando interfaces de rede: %v\n", err)
	}

	defIface, err := netinfo.DefaultInterface(ifaces, netinfo.NewUDPRouteSource())
	if err != nil {
		if len(ifaces) > 0 {
			defIface = ifaces[0]
		} else {
			log.Fatalf("Nenhuma interface de rede ativa encontrada: %v\n", err)
		}
	}

	fmt.Printf("[OK] Interface: %s (IP: %s, Sub-rede: %s)\n", defIface.Name, defIface.Addr, defIface.Prefix)

	// 3. Determine target CIDR
	cidr := defIface.Prefix.String()
	if len(os.Args) > 1 {
		cidr = os.Args[1]
	}
	fmt.Printf("[OK] Alvo da Varredura: %s\n\n", cidr)

	// 4. Initialize scanner components
	ouiDB, err := oui.Load()
	if err != nil {
		log.Fatalf("Erro carregando base OUI: %v\n", err)
	}

	var pinger icmp.Pinger
	pinger, err = icmp.NewPinger()
	if err != nil {
		pinger = nil
	} else {
		defer pinger.Close()
	}

	arpEngine := arp.NewEngine(nil, nil, ouiDB, defIface)
	orch := scanner.NewOrchestrator(pinger, arpEngine, ouiDB)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var discovered []scanner.DiscoveredHost

	handlers := scanner.SweepHandlers{
		OnProgress: func(p scanner.SweepProgressEvent) {
			fmt.Printf("\r[Progresso] %-10s | %5.1f%% | Escaneados: %-4d | Ativos: %-3d (ICMP: %-3d, ARP: %-3d)",
				p.Stage, p.Percent, p.ScannedHosts, p.ActiveHosts, p.ICMPDiscovered, p.ARPDiscovered)
			if p.Stage == scanner.StageFinished || p.Stage == scanner.StageError {
				fmt.Println()
			}
		},
		OnBatch: func(b scanner.HostBatchEvent) {
			discovered = append(discovered, b.Hosts...)
		},
		OnSummary: func(s scanner.SweepSummaryEvent) {
			fmt.Printf("\n[Sucesso] Varredura finalizada em %v.\n", s.Duration.Round(time.Millisecond))
		},
	}

	cfg := scanner.OrchestratorConfig{
		CIDR:        cidr,
		Interface:   defIface,
		RateLimit:   100,
		ICMPTimeout: 1000 * time.Millisecond,
		ARPTimeout:  1500 * time.Millisecond,
	}

	fmt.Println("Iniciando varredura... Pressione Ctrl+C para cancelar.")
	start := time.Now()
	runErr := orch.Run(ctx, cfg, handlers)
	if runErr != nil && ctx.Err() == nil {
		log.Fatalf("\nErro na varredura: %v\n", runErr)
	}

	// 5. Print results table
	fmt.Println("\n=========================================================================================")
	fmt.Printf("%-16s | %-17s | %-30s | %-10s | %s\n", "ENDEREÇO IP", "ENDEREÇO MAC", "FABRICANTE OUI", "LATÊNCIA", "MÉTODO")
	fmt.Println("-----------------------------------------------------------------------------------------")

	if len(discovered) == 0 {
		fmt.Println("Nenhum host ativo encontrado.")
	} else {
		for _, h := range discovered {
			vendor := h.Vendor
			if h.Kind == "private" {
				vendor = "[MAC Privado/Aleatório]"
			} else if vendor == "" {
				vendor = "Não identificado"
			}
			if len(vendor) > 30 {
				vendor = vendor[:27] + "..."
			}

			rttStr := "-"
			if h.Responded {
				rttStr = fmt.Sprintf("%.2f ms", float64(h.RTT.Microseconds())/1000.0)
			}

			macStr := h.MAC
			if macStr == "" {
				macStr = "Desconhecido"
			}

			fmt.Printf("%-16s | %-17s | %-30s | %-10s | %s\n",
				h.IP, macStr, vendor, rttStr, h.DiscoveredBy)
		}
	}
	fmt.Println("=========================================================================================")
	fmt.Printf("Total de dispositivos ativos: %d (Tempo total: %v)\n", len(discovered), time.Since(start).Round(time.Millisecond))
}
