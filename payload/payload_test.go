package payload

import (
	"testing"

	"github.com/polarsignals/kubezonnet/byteorder"
	"github.com/stretchr/testify/require"
)

func TestPayloadEncodeDecode(t *testing.T) {
	inputKeys := []IPKey{
		{SrcIP: byteorder.Htonl(1), DstIP: byteorder.Htonl(2), SrcPort: 80, DstPort: 443},
		{SrcIP: byteorder.Htonl(4), DstIP: byteorder.Htonl(5), SrcPort: 8080, DstPort: 8443},
	}
	inputValues := []IPValue{{PacketSize: 3}, {PacketSize: 6}}
	buf := Encode(inputKeys, inputValues)

	entries, err := Decode(buf)
	require.NoError(t, err)

	// Decode converts from network byte order to host byte order
	expectedKeys := []IPKey{
		{SrcIP: 1, DstIP: 2, SrcPort: 80, DstPort: 443},
		{SrcIP: 4, DstIP: 5, SrcPort: 8080, DstPort: 8443},
	}
	expectedValues := []IPValue{{PacketSize: 3}, {PacketSize: 6}}

	resKeys := []IPKey{}
	resValues := []IPValue{}
	for _, entry := range entries {
		resKeys = append(resKeys, IPKey{SrcIP: entry.SrcIP, DstIP: entry.DstIP, SrcPort: entry.SrcPort, DstPort: entry.DstPort})
		resValues = append(resValues, IPValue{PacketSize: entry.Traffic})
	}

	require.Equal(t, expectedKeys, resKeys)
	require.Equal(t, expectedValues, resValues)
}
