package payload

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPayloadEncodeDecode(t *testing.T) {
	inputKeys := []IPKey{
		{SrcIP: 1, DstIP: 2},
		{SrcIP: 4, DstIP: 5},
	}
	inputValues := []IPValue{{PacketSize: 3}, {PacketSize: 6}}
	buf := Encode(inputKeys, inputValues)

	entries, err := Decode(buf)
	require.NoError(t, err)

	resKeys := []IPKey{}
	resValues := []IPValue{}
	for _, entry := range entries {
		resKeys = append(resKeys, IPKey{SrcIP: entry.SrcIP, DstIP: entry.DstIP})
		resValues = append(resValues, IPValue{PacketSize: entry.Traffic})
	}

	require.Equal(t, inputKeys, resKeys)
	require.Equal(t, inputValues, resValues)
}
