package http

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/sirupsen/logrus"
)

type HttpMethod uint8

const (
	HTTP_METHOD_UNKNOWN HttpMethod = iota
	HttpGet
	HttpPost
	HttpPut
	HttpDelete
	HttpHead
	HttpOptions
	HttpPatch
)

func (m HttpMethod) String() string {
	switch m {
	case HttpGet:
		return "GET"
	case HttpPost:
		return "POST"
	case HttpPut:
		return "PUT"
	case HttpDelete:
		return "DELETE"
	case HttpHead:
		return "HEAD"
	case HttpOptions:
		return "OPTIONS"
	case HttpPatch:
		return "PATCH"
	default:
		return "UNKNOWN"
	}
}

const (
	programPath = "target/http.bpf.o"
	programName = "socket__filter_package"
	mapFilter   = "filter_map"
	mapMetric   = "metrics_map"
)

type Interface interface {
	Load() error
	Close() error
}

type ConnTuple struct {
	DestIP     [4]byte
	DestPort   uint16
	SourceIP   [4]byte
	SourcePort uint16
}

type HttpPackage struct {
	RequestTimestamp uint64
	Duration         uint64
	StatusCode       uint16
	Method           HttpMethod
	RequestFragment  [HttpPayloadSize]byte
}

func (p HttpPackage) String() string {
	return fmt.Sprintf("timestamp: %d, duration: %d, status: %d, method: %d, request: %s",
		p.RequestTimestamp, p.Duration, p.StatusCode, p.Method, string(p.RequestFragment[:]))
}

func (p HttpPackage) Output(conn ConnTuple) string {
	return fmt.Sprintf("src: %s:%d, dst: %s:%d, timestamp: %d, duration(ms): %d, status: %d, method: %s, path: %s",
		net.IP(conn.SourceIP[:]).String(), conn.SourcePort, net.IP(conn.DestIP[:]).String(), conn.DestPort, p.RequestTimestamp, p.Duration/1e6, p.StatusCode, p.Method, p.GetPath())
}

func (p HttpPackage) GetPath() string {
	list := strings.Split(string(p.RequestFragment[:]), " ")
	if len(list) >= 1 {
		uri, err := url.ParseRequestURI(list[0])
		if err == nil {
			return uri.Path
		}
		return strings.TrimRight(list[0], "?")
	}
	return ""
}

func (p HttpPackage) GetRoute() string {
	return p.Method.String() + " " + p.GetPath()
}

func (p HttpPackage) GetDuration() int64 {
	return int64(p.Duration)
}

const (
	HttpPayloadSize = 224
)

type provider struct {
	ifIndex    int
	ipAddress  string
	collection *ebpf.Collection
	fd         int
	sock       int
	log        logrus.Logger
	ch         chan *HttpPackage
}

const (
	SO_ATTACH_BPF = 0x32                     // 50
	SO_DETACH_BPF = syscall.SO_DETACH_FILTER // 27
	ProtocolICMP  = 1                        // Internet Control Message
)

func New(log logrus.Logger, ifIndex int, ip string, ch chan *HttpPackage) Interface {
	return &provider{
		log:       log,
		ifIndex:   ifIndex,
		ipAddress: ip,
		ch:        ch,
	}
}
func (e *provider) Load() error {
	if err := rlimit.RemoveMemlock(); err != nil {
		e.log.Fatal(err)
	}
	programBytes, err := os.ReadFile(programPath)
	if err != nil {
		return err
	}
	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(programBytes))
	if err != nil {
		return err
	}
	e.collection, err = ebpf.NewCollectionWithOptions(spec, ebpf.CollectionOptions{})
	if err != nil {
		return err
	}
	program := e.collection.DetachProgram(programName)
	if program == nil {
		return fmt.Errorf("detach program %s failed", programName)
	}
	e.fd = program.FD()
	e.sock, err = OpenRawSock(e.ifIndex)
	if err != nil {
		return err
	}
	if err := syscall.SetsockoptInt(e.sock, syscall.SOL_SOCKET, SO_ATTACH_BPF, program.FD()); err != nil {
		return err
	}
	m := e.collection.DetachMap(mapMetric)
	go e.FanInMetric(m)
	return nil
}
func (e *provider) FanInMetric(m *ebpf.Map) {
	defer func() {
		if err := recover(); err != nil {
			e.log.Errorf("panic: %v", err)
			e.log.Errorf("stack: %s", string(debug.Stack()))
		}
	}()
	var (
		key []byte
		val HttpPackage
	)
	for {
		for m.Iterate().Next(&key, &val) {
			// clean map
			if err := m.Delete(key); err != nil {
				e.log.Errorf("delete map error: %v", err)
				continue
			}
			conn := ConnTuple{}
			if err := binary.Read(bytes.NewReader(key), binary.LittleEndian, &conn); err != nil {
				e.log.Errorf("decode conn error: %v", err)
				continue
			}
			e.log.Infof("%s", val.Output(conn))
			e.ch <- &val
		}
		time.Sleep(1 * time.Second)
	}
}
func (e *provider) Close() error {
	_ = syscall.SetsockoptInt(e.sock, syscall.SOL_SOCKET, SO_DETACH_BPF, e.fd)
	e.collection.Close()
	return nil
}
