package sfu

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lucsky/cuid"
	"github.com/pion/ion-sfu/pkg/log"
)

var (
	errQuicConnectionInitFailed = errors.New("relay init failed")
)

// QuicConfig represents the configuration of a quic session
type QuicConfig struct {
	Addr     string
	CertFile string
	KeyFile  string
}

// Quic represents a sfu peer connection
type Quic struct {
	id                 string
	l                  quic.Listener
	mu                 sync.RWMutex
	stop               bool
	routers            map[uint32]*Router
	routersLock        sync.RWMutex
	onCloseHandler     func()
	onRouterHander     func(*Router)
	onRouterHanderLock sync.RWMutex
}

func getDefaultQuicConfig() *quic.Config {
	return &quic.Config{
		MaxIncomingStreams:                    1000,
		MaxIncomingUniStreams:                 -1,              // disable unidirectional streams
		MaxReceiveStreamFlowControlWindow:     3 * (1 << 20),   // 3 MB
		MaxReceiveConnectionFlowControlWindow: 4.5 * (1 << 20), // 4.5 MB
		KeepAlive:                             true,
	}
}

// NewQuic creates a new Quic
func NewQuic(config *QuicConfig) (*Quic, error) {
	q := &Quic{
		id:      cuid.New(),
		routers: make(map[uint32]*Router),
	}

	err := http3.ListenAndServeQUIC(config.Addr, config.CertFile, config.KeyFile, http.HandlerFunc(q.handler))

	if err != nil {
		log.Errorf("Error initializing quic server: %s", err)
		return nil, err
	}

	http.Client{
		Transport: &http3.RoundTripper{},
	}

	// pc.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
	// 	log.Debugf("Peer %s got remote track %v", q.id, track)
	// 	var recv Receiver
	// 	switch track.Kind() {
	// 	case webrtc.RTPCodecTypeVideo:
	// 		recv = NewVideoReceiver(config.Receiveq.Video, track)
	// 	case webrtc.RTPCodecTypeAudio:
	// 		recv = NewAudioReceiver(track)
	// 	}

	// 	if recv.Track().Kind() == webrtc.RTPCodecTypeVideo {
	// 		go q.sendRTCP(recv)
	// 	}

	// 	router := NewRouter(recv)

	// 	q.routersLock.Lock()
	// 	q.routers[recv.Track().SSRC()] = router
	// 	q.routersLock.Unlock()

	// 	log.Debugf("Create router %s %d", q.id, recv.Track().SSRC())

	// 	q.onRouterHanderLock.Lock()
	// 	defer q.onRouterHanderLock.Unlock()
	// 	if q.onRouterHander != nil {
	// 		q.onRouterHander(router)
	// 	}
	// })

	return q, nil
}

func (q *Quic) handler(w http.ResponseWriter, r *http.Request) {
	log.Infof("called handler")
}

// OnClose is called when the peer is closed
func (q *Quic) OnClose(f func()) {
	q.onCloseHandler = f
}

// OnRouter handler called when a router is added
func (q *Quic) OnRouter(f func(*Router)) {
	q.onRouterHanderLock.Lock()
	q.onRouterHander = f
	q.onRouterHanderLock.Unlock()
}

// Subscribe to a router
// `renegotiate` flag is supported until pion/webrtc supports
// OnNegotiationNeeded (https://github.com/pion/webrtc/pull/1322)
func (q *Quic) Subscribe(router *Router, renegotiate bool) error {
	log.Infof("Subscribing to router %v", router)

	// track := routeq.pub.Track()
	// to := q.me.GetCodecsByName(track.Codec().Name)

	// if len(to) == 0 {
	// 	log.Errorf("Error mapping payload type")
	// 	return errPtNotSupported
	// }

	// pt := to[0].PayloadType

	// log.Debugf("Creating track: %d %d %s %s", pt, track.SSRC(), track.ID(), track.Label())
	// track, err := q.pc.NewTrack(pt, track.SSRC(), track.ID(), track.Label())

	// if err != nil {
	// 	log.Errorf("Error creating track")
	// 	return err
	// }

	// s, err := q.pc.AddTrack(track)

	// if err != nil {
	// 	log.Errorf("Error adding send track")
	// 	return err
	// }

	// // Create webrtc sender for the peer we are sending track to
	// sender := NewWebRTCSender(track, s)

	// // Attach sender to source
	// routeq.AddSub(q.id, sender)

	return nil
}

// AddSub adds peer as a sub
func (q *Quic) AddSub(transport Transport) {
	q.routersLock.Lock()
	for _, router := range q.routers {
		err := transport.Subscribe(router, false)
		if err != nil {
			log.Errorf("Error subscribing transport %s to router %v", transport.ID(), router)
		}
	}
	q.routersLock.Unlock()
}

// ID of peer
func (q *Quic) ID() string {
	return q.id
}

// Routers returns routers for this peer
func (q *Quic) Routers() map[uint32]*Router {
	q.routersLock.RLock()
	defer q.routersLock.RUnlock()
	return q.routers
}

// Close peer
func (q *Quic) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.stop {
		return nil
	}

	// q.routersLock.Lock()
	// for _, router := range q.routers {
	// 	routeq.Close()
	// }
	// q.routersLock.Unlock()

	// if q.onCloseHandler != nil {
	// 	q.onCloseHandler()
	// }
	// q.stop = true
	return nil
}

func (q *Quic) stats() string {
	info := fmt.Sprintf("  peer: %s\n", q.id)

	q.routersLock.RLock()
	for ssrc, router := range q.routers {
		info += fmt.Sprintf("    router: %d | %s\n", ssrc, router.pub.stats())

		if len(router.subs) < 6 {
			for pid, sub := range router.subs {
				info += fmt.Sprintf("      sub: %s | %s\n", pid, sub.stats())
			}
			info += "\n"
		} else {
			info += fmt.Sprintf("      subs: %d\n\n", len(router.subs))
		}
	}
	q.routersLock.RUnlock()
	return info
}
