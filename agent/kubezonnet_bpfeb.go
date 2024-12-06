// Code generated by bpf2go; DO NOT EDIT.
//go:build mips || mips64 || ppc64 || s390x

package agent

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type kubezonnetIpKey struct {
	SrcIp  uint32
	DestIp uint32
}

type kubezonnetIpValue struct{ PacketSize uint64 }

// loadKubezonnet returns the embedded CollectionSpec for kubezonnet.
func loadKubezonnet() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_KubezonnetBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load kubezonnet: %w", err)
	}

	return spec, err
}

// loadKubezonnetObjects loads kubezonnet and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*kubezonnetObjects
//	*kubezonnetPrograms
//	*kubezonnetMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func loadKubezonnetObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := loadKubezonnet()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// kubezonnetSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type kubezonnetSpecs struct {
	kubezonnetProgramSpecs
	kubezonnetMapSpecs
}

// kubezonnetSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type kubezonnetProgramSpecs struct {
	NfPostroutingHook *ebpf.ProgramSpec `ebpf:"nf_postrouting_hook"`
}

// kubezonnetMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type kubezonnetMapSpecs struct {
	IpMap *ebpf.MapSpec `ebpf:"ip_map"`
}

// kubezonnetObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadKubezonnetObjects or ebpf.CollectionSpec.LoadAndAssign.
type kubezonnetObjects struct {
	kubezonnetPrograms
	kubezonnetMaps
}

func (o *kubezonnetObjects) Close() error {
	return _KubezonnetClose(
		&o.kubezonnetPrograms,
		&o.kubezonnetMaps,
	)
}

// kubezonnetMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to loadKubezonnetObjects or ebpf.CollectionSpec.LoadAndAssign.
type kubezonnetMaps struct {
	IpMap *ebpf.Map `ebpf:"ip_map"`
}

func (m *kubezonnetMaps) Close() error {
	return _KubezonnetClose(
		m.IpMap,
	)
}

// kubezonnetPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadKubezonnetObjects or ebpf.CollectionSpec.LoadAndAssign.
type kubezonnetPrograms struct {
	NfPostroutingHook *ebpf.Program `ebpf:"nf_postrouting_hook"`
}

func (p *kubezonnetPrograms) Close() error {
	return _KubezonnetClose(
		p.NfPostroutingHook,
	)
}

func _KubezonnetClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed kubezonnet_bpfeb.o
var _KubezonnetBytes []byte
