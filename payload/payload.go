package payload

import (
	"encoding/binary"
	"errors"

	"github.com/polarsignals/kubezonnet/byteorder"
)

// Key for the eBPF map representing IP pairs
type IPKey struct {
	SrcIP    uint32
	DstIP    uint32
	SrcPort  uint16
	DstPort  uint16
}

// Value for the eBPF map representing total packet sizes
type IPValue struct {
	PacketSize uint64
}

func Encode(keys []IPKey, values []IPValue) []byte {
	size := 4 + 20*len(keys) // The first 4 bytes encode the length, then 2 uint32s, 2 uint16s, and 1 uint64 per entry in the data.
	buf := make([]byte, size)

	binary.BigEndian.PutUint32(buf[:4], uint32(len(keys)))
	offset := 4

	for i, srcDst := range keys {
		binary.BigEndian.PutUint32(buf[offset:offset+4], srcDst.SrcIP)
		binary.BigEndian.PutUint32(buf[offset+4:offset+8], srcDst.DstIP)
		binary.BigEndian.PutUint16(buf[offset+8:offset+10], srcDst.SrcPort)
		binary.BigEndian.PutUint16(buf[offset+10:offset+12], srcDst.DstPort)
		binary.BigEndian.PutUint64(buf[offset+12:offset+20], values[i].PacketSize)
		offset += 20
	}

	return buf
}

type Entry struct {
	SrcIP   uint32
	DstIP   uint32
	SrcPort uint16
	DstPort uint16
	Traffic uint64
}

func Decode(buf []byte) ([]Entry, error) {
	if len(buf) < 4 {
		return nil, errors.New("unexpected length of buffer")
	}
	numEntries := binary.BigEndian.Uint32(buf[:4])

	size := 4 + 20*numEntries // The first 4 bytes encode the length, then 2 uint32s, 2 uint16s, and 1 uint64 per entry in the data.
	if uint32(len(buf)) != size {
		return nil, errors.New("unexpected length of buffer for number of entries")
	}

	entries := make([]Entry, numEntries)
	for i := uint32(0); i < numEntries; i++ {
		offset := 4 + i*20
		srcIP := binary.BigEndian.Uint32(buf[offset : offset+4])
		dstIP := binary.BigEndian.Uint32(buf[offset+4 : offset+8])
		srcPort := binary.BigEndian.Uint16(buf[offset+8 : offset+10])
		dstPort := binary.BigEndian.Uint16(buf[offset+10 : offset+12])
		traffic := binary.BigEndian.Uint64(buf[offset+12 : offset+20])
		entries[i] = Entry{
			SrcIP:   byteorder.Ntohl(srcIP),
			DstIP:   byteorder.Ntohl(dstIP),
			SrcPort: srcPort,
			DstPort: dstPort,
			Traffic: traffic,
		}
	}

	return entries, nil
}
