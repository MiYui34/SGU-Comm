package whip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
)

// Settings: WebRTC for WHIP on one UDP port that every session shares
// (one entry in a firewall or a compose file). publicIP, if set, replaces
// the server's own addresses in its ICE candidates (behind Docker's NAT).
// No STUN, TURN or ICE-TCP: the server has a public address, and OBS
// (libjuice) speaks ICE over UDP only. IPv4 only: FFmpeg's WHIP client
// tries the first candidate alone (and cannot reach IPv6 ones).
func Settings(mux ice.UDPMux, publicIP net.IP) (webrtc.SettingEngine, error) {
	var s webrtc.SettingEngine
	s.SetICEUDPMux(mux)
	s.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	s.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	// A peer that is gone is noticed in seconds (the defaults take 30).
	s.SetICETimeouts(5*time.Second, 10*time.Second, 2*time.Second)
	if publicIP == nil {
		return s, nil
	}
	return s, s.SetICEAddressRewriteRules(webrtc.ICEAddressRewriteRule{
		External: []string{publicIP.String()}, AsCandidateType: webrtc.ICECandidateTypeHost,
	})
}

// Listen opens the UDP port on every IPv4 interface. publicHost: the
// public IPv4 address, or a name to resolve now; "": the interfaces' own
// addresses.
func Listen(port int, publicHost string) (webrtc.SettingEngine, io.Closer, error) {
	var ip net.IP
	if publicHost != "" {
		var err error
		if ip, err = resolve(publicHost); err != nil {
			return webrtc.SettingEngine{}, nil, fmt.Errorf("WHIP_PUBLIC_IP: %w", err)
		}
	}
	mux, err := ice.NewMultiUDPMuxFromPort(port, ice.UDPMuxFromPortWithNetworks(ice.NetworkTypeUDP4))
	if err != nil {
		return webrtc.SettingEngine{}, nil, fmt.Errorf("WHIP UDP port %d: %w", port, err)
	}
	s, err := Settings(mux, ip)
	if err != nil {
		mux.Close()
		return webrtc.SettingEngine{}, nil, err
	}
	return s, mux, nil
}

func resolve(host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return nil, errors.New("an IPv4 address, please")
		}
		return ip, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil, err
	}
	return addrs[0], nil
}
