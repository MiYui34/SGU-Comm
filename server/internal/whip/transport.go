package whip

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
)

// Settings: WebRTC for WHIP on one UDP port that every session shares
// (one entry in a firewall or a compose file). publicIPs, if any, replace
// the server's own addresses in its ICE candidates (behind Docker's NAT).
// No STUN, TURN or ICE-TCP: the server has a public address, and OBS
// (libjuice) speaks ICE over UDP only.
func Settings(mux ice.UDPMux, publicIPs []net.IP) (webrtc.SettingEngine, error) {
	var s webrtc.SettingEngine
	s.SetICEUDPMux(mux)
	s.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4, webrtc.NetworkTypeUDP6})
	s.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	// A peer that is gone is noticed in seconds (the defaults take 30).
	s.SetICETimeouts(5*time.Second, 10*time.Second, 2*time.Second)
	var rules []webrtc.ICEAddressRewriteRule
	for _, ip := range publicIPs {
		network := webrtc.NetworkTypeUDP6
		if ip.To4() != nil {
			network = webrtc.NetworkTypeUDP4
		}
		rules = append(rules, webrtc.ICEAddressRewriteRule{
			External: []string{ip.String()}, AsCandidateType: webrtc.ICECandidateTypeHost, Networks: []webrtc.NetworkType{network},
		})
	}
	return s, s.SetICEAddressRewriteRules(rules...)
}

// Listen opens the UDP port on every interface. publicHost: the public
// IP, or a name to resolve now (its first IPv4 and IPv6 address); "":
// the interfaces' own addresses.
func Listen(port int, publicHost string) (webrtc.SettingEngine, io.Closer, error) {
	var ips []net.IP
	if publicHost != "" {
		var err error
		if ips, err = resolve(publicHost); err != nil {
			return webrtc.SettingEngine{}, nil, fmt.Errorf("WHIP_PUBLIC_IP: %w", err)
		}
	}
	mux, err := ice.NewMultiUDPMuxFromPort(port)
	if err != nil {
		return webrtc.SettingEngine{}, nil, fmt.Errorf("WHIP UDP port %d: %w", port, err)
	}
	s, err := Settings(mux, ips)
	if err != nil {
		mux.Close()
		return webrtc.SettingEngine{}, nil, err
	}
	return s, mux, nil
}

func resolve(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var v4, v6 net.IP
	for _, a := range addrs {
		if a.IP.To4() != nil && v4 == nil {
			v4 = a.IP
		} else if a.IP.To4() == nil && v6 == nil {
			v6 = a.IP
		}
	}
	var out []net.IP
	for _, ip := range []net.IP{v4, v6} {
		if ip != nil {
			out = append(out, ip)
		}
	}
	return out, nil
}
