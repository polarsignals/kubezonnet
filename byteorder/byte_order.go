package byteorder

func Htonl(x uint32) uint32 {
	return (x << 24) | ((x << 8) & 0x00FF0000) |
		((x >> 8) & 0x0000FF00) | (x >> 24)
}

func Ntohl(x uint32) uint32 {
	return (x << 24) | ((x << 8) & 0x00FF0000) |
		((x >> 8) & 0x0000FF00) | (x >> 24)
}
