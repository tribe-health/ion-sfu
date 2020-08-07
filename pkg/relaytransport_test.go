package sfu

import (
	"testing"

	"github.com/pion/ion-sfu/pkg/relay"
	"github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/assert"
)

func TestRelayTransportOpenClose(t *testing.T) {
	sessionID := uint32(1)
	session := NewSession(sessionID)
	client := relay.NewClient(sessionID, "localhost:5558")
	assert.NotNil(t, client)

	relay, err := NewRelayTransport(session, client)
	assert.NoError(t, err)

	relay.Close()
}

func TestRelayTransport(t *testing.T) {
	sessionID := uint32(1)
	session := NewSession(sessionID)
	server := relay.NewServer(5559)
	assert.NotNil(t, server)
	client := relay.NewClient(sessionID, "localhost:5559")
	assert.NotNil(t, client)

	relay, err := NewRelayTransport(session, client)
	assert.NoError(t, err)

	assert.NotNil(t, relay.ID())

	done := make(chan struct{})
	go func() {
		conn := server.AcceptSession()
		assert.Equal(t, conn.ID(), sessionID)
		close(done)
	}()

	ssrc := uint32(5000)
	track, err := webrtc.NewTrack(webrtc.DefaultPayloadTypeOpus, ssrc, "audio", "pion", webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))
	assert.NoError(t, err)

	sender, err := relay.NewSender(track)
	assert.NoError(t, err)

	sendRTPWithSenderUntilDone(done, t, track, sender)
	relay.Close()
}
