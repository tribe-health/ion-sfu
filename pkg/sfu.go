package sfu

import (
	"sync"
	"time"

	"github.com/pion/webrtc/v3"

	"github.com/pion/ion-sfu/pkg/log"
	"github.com/pion/ion-sfu/pkg/relay"
)

// ICEServerConfig defines parameters for ice servers
type ICEServerConfig struct {
	URLs       []string `mapstructure:"urls"`
	Username   string   `mapstructure:"username"`
	Credential string   `mapstructure:"credential"`
}

// WebRTCConfig defines parameters for ice
type WebRTCConfig struct {
	ICEPortRange []uint16          `mapstructure:"portrange"`
	ICEServers   []ICEServerConfig `mapstructure:"iceserver"`
}

// RelayConfig server configuration
type RelayConfig struct {
	Port int `mapstructure:"port"`
}

// ReceiverConfig defines receiver configurations
type ReceiverConfig struct {
	Video WebRTCVideoReceiverConfig `mapstructure:"video"`
}

// Config for base SFU
type Config struct {
	WebRTC   WebRTCConfig   `mapstructure:"webrtc"`
	Log      log.Config     `mapstructure:"log"`
	Receiver ReceiverConfig `mapstructure:"receiver"`
	Relay    RelayConfig    `mapstructure:"relay"`
}

var (
	// only support unified plan
	cfg = webrtc.Configuration{
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
	}

	config  Config
	setting webrtc.SettingEngine
)

// SFU represents an sfu instance
type SFU struct {
	stop     bool
	mu       sync.RWMutex
	relay    *relay.Server
	sessions map[uint32]*Session
}

// NewSFU creates a new sfu instance
func NewSFU(c Config) *SFU {
	s := &SFU{
		sessions: make(map[uint32]*Session),
	}

	config = c

	log.Init(c.Log.Level)

	var icePortStart, icePortEnd uint16

	if len(c.WebRTC.ICEPortRange) == 2 {
		icePortStart = c.WebRTC.ICEPortRange[0]
		icePortEnd = c.WebRTC.ICEPortRange[1]
	}

	if icePortStart != 0 || icePortEnd != 0 {
		if err := setting.SetEphemeralUDPPortRange(icePortStart, icePortEnd); err != nil {
			panic(err)
		}
	}

	var iceServers []webrtc.ICEServer
	for _, iceServer := range c.WebRTC.ICEServers {
		s := webrtc.ICEServer{
			URLs:       iceServer.URLs,
			Username:   iceServer.Username,
			Credential: iceServer.Credential,
		}
		iceServers = append(iceServers, s)
	}

	cfg.ICEServers = iceServers

	if c.Relay.Port != 0 {
		log.Infof("Relay server listening at %d", c.Relay.Port)
		s.relay = relay.NewServer(c.Relay.Port)
		go s.acceptRelay()
	}

	go s.stats()

	return s
}

func (s *SFU) acceptRelay() {
	for {
		if s.stop {
			break
		}

		conn := s.relay.AcceptSession()

		sessionID := conn.ID()
		session := s.getSession(sessionID)
		if session == nil {
			session = s.newSession(sessionID)
		}

		_, err := NewRelayTransport(session, conn)
		if err != nil {
			log.Errorf("Error creating RelayTransport: %s", err)
		}
	}
}

// NewSession creates a new session instance
func (s *SFU) newSession(id uint32) *Session {
	session := NewSession(id)
	session.OnClose(func() {
		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()
	})

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()
	return session
}

// GetSession by id
func (s *SFU) getSession(id uint32) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

// NewWebRTCTransport creates a new WebRTCTransport that is a member of a session
func (s *SFU) NewWebRTCTransport(sid uint32, offer webrtc.SessionDescription) (*WebRTCTransport, error) {
	session := s.getSession(sid)

	if session == nil {
		session = s.newSession(sid)
	}

	t, err := NewWebRTCTransport(session, offer)
	if err != nil {
		return nil, err
	}

	session.AddTransport(t)

	return t, nil
}

// NewRelayTransport creates a new RelayTransport that can be used to relay streams between nodes
func (s *SFU) NewRelayTransport(sid uint32, addr string) (*RelayTransport, error) {
	session := s.getSession(sid)

	if session == nil {
		session = s.newSession(sid)
	}

	conn := relay.NewClient(sid, addr)

	t, err := NewRelayTransport(session, conn)
	if err != nil {
		return nil, err
	}

	return t, nil
}

// Close shutsdown the sfu instance
func (s *SFU) Close() {
	s.relay.Close()
}

func (s *SFU) stats() {
	t := time.NewTicker(statCycle)
	for range t.C {
		info := "\n----------------stats-----------------\n"

		s.mu.RLock()
		if len(s.sessions) == 0 {
			s.mu.RUnlock()
			continue
		}

		for _, session := range s.sessions {
			info += session.stats()
		}
		s.mu.RUnlock()
		log.Infof(info)
	}
}
