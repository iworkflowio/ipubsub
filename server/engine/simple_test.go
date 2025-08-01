package engine

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicSendReceive(t *testing.T) {
	stream := NewInMemoryStreamImpl(5)
	defer stream.Stop()

	// Test circular buffer mode
	output := OutputType{"message": "test"}
	uid := uuid.New()
	timestamp := time.Now()

	isTimeout, err := stream.Send(output, uid, timestamp, 0) // circular buffer mode
	require.NoError(t, err)
	assert.False(t, isTimeout)

	resp, isTimeout, err := stream.Receive(1)
	require.NoError(t, err)
	assert.False(t, isTimeout)
	assert.Equal(t, uid.String(), resp.OutputUuid)
	assert.Equal(t, output, resp.Output)
}

func TestBlockingMode(t *testing.T) {
	stream := NewInMemoryStreamImpl(1)
	defer stream.Stop()

	// Fill the buffer
	isTimeout, err := stream.Send(OutputType{"message": "fill"}, uuid.New(), time.Now(), 5)
	require.NoError(t, err)
	assert.False(t, isTimeout)

	// This should timeout
	start := time.Now()
	isTimeout, err = stream.Send(OutputType{"message": "should timeout"}, uuid.New(), time.Now(), 1)
	duration := time.Since(start)

	require.Error(t, err)
	assert.True(t, isTimeout)
	assert.Contains(t, err.Error(), "timeout waiting for stream space")
	assert.GreaterOrEqual(t, duration, 900*time.Millisecond)
}
