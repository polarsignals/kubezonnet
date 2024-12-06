package byteorder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHelloWorld(t *testing.T) {
	hostValue := uint32(0x12345678)
	netValue := Htonl(hostValue)
	require.Equal(t, uint32(0x78563412), netValue)

	hostAgain := Ntohl(netValue)
	require.Equal(t, hostValue, hostAgain)
}
