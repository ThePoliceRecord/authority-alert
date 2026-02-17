package handler

import (
	"fmt"
	"math/rand"
	"net"
	"sort"

	"supervisor/pkg/logger"
)

const (
	relaySRVRecord = "_camera._websocket._tcp._aarelay.thepolicerecord.com"
)

// relayServer represents a discovered relay server from SRV records
type relayServer struct {
	Host     string
	Port     uint16
	Priority uint16
	Weight   uint16
}

// discoverRelayServers resolves SRV records and returns servers sorted by priority
func discoverRelayServers() ([]relayServer, error) {
	_, addrs, err := net.LookupSRV("", "", relaySRVRecord)
	if err != nil {
		return nil, fmt.Errorf("SRV lookup failed: %w", err)
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no SRV records found for %s", relaySRVRecord)
	}

	servers := make([]relayServer, 0, len(addrs))
	for _, addr := range addrs {
		// Remove trailing dot from hostname
		host := addr.Target
		if len(host) > 0 && host[len(host)-1] == '.' {
			host = host[:len(host)-1]
		}
		servers = append(servers, relayServer{
			Host:     host,
			Port:     addr.Port,
			Priority: addr.Priority,
			Weight:   addr.Weight,
		})
	}

	// Sort by priority (lower is better), then by weight (higher is better for selection)
	sort.Slice(servers, func(i, j int) bool {
		if servers[i].Priority != servers[j].Priority {
			return servers[i].Priority < servers[j].Priority
		}
		return servers[i].Weight > servers[j].Weight
	})

	logger.Info("Discovered %d relay servers via SRV", len(servers))
	for i, s := range servers {
		logger.Debug("  [%d] %s:%d (priority=%d, weight=%d)", i, s.Host, s.Port, s.Priority, s.Weight)
	}

	return servers, nil
}

// selectServerByWeight selects a server from same-priority group using weighted random selection
func selectServerByWeight(servers []relayServer) relayServer {
	if len(servers) == 1 {
		return servers[0]
	}

	// Calculate total weight (treat 0 weight as 1 for selection purposes)
	totalWeight := 0
	for _, s := range servers {
		w := int(s.Weight)
		if w == 0 {
			w = 1
		}
		totalWeight += w
	}

	// Random selection based on weight
	r := rand.Intn(totalWeight)
	for _, s := range servers {
		w := int(s.Weight)
		if w == 0 {
			w = 1
		}
		r -= w
		if r < 0 {
			return s
		}
	}

	// Fallback to first server
	return servers[0]
}

// getServersForPriority returns all servers with the given priority
func getServersForPriority(servers []relayServer, priority uint16) []relayServer {
	result := make([]relayServer, 0)
	for _, s := range servers {
		if s.Priority == priority {
			result = append(result, s)
		}
	}
	return result
}

// buildRelayURL constructs a secure WebSocket URL from a relay server
func buildRelayURL(server relayServer) string {
	return fmt.Sprintf("wss://%s:%d/ws", server.Host, server.Port)
}
