package tcp

import (
	"bytes"
	ctx "context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pkg/tcp/context"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"

	"github.com/pkg/errors"
)

var (
	_tcpDSN string
)

func init() {
	addFlag(flag.CommandLine)
}

func addFlag(fs *flag.FlagSet) {
	v := os.Getenv("TCP")
	if v == "" {
		v = "tcp://0.0.0.0:8099"
	}
	fs.StringVar(&_tcpDSN, "tcp", v, "listen go tcp dsn")
}

func parseDSN(rawdsn string) (*ServerConfig, error) {
	rawdsn = os.ExpandEnv(rawdsn)
	u, err := url.Parse(rawdsn)
	return &ServerConfig{Addr: u.String(), Network: "tcp"}, err
}

// ServerConfig tcp server settings.
type ServerConfig struct {
	Network string `dsn:"network"`
	Addr    string `dsn:"address"`
}

// Server represents an TCP Server.
type Server struct {
	lis        net.Listener
	serviceMap map[string]*service

	lg *zap.Logger
}

// service 服务
type service struct {
	method string
	Arg    *ProtoBuff
	Reply  *ProtoBuff
	fn     func(c ctx.Context, arg *ProtoBuff) (*ProtoBuff, error)
}

type serverCodec struct {
	sending sync.Mutex
	req     Request
	resp    ProtoHeader
	auth    string

	rwc    io.ReadWriteCloser
	addr   net.Addr
	closed bool
}

type Request struct {
	ProtoHeader
	timeout time.Duration // timeout
	ctx     context.Context
}

var DefaultServer = newServer()

// NewServer new a tcp server.
func NewServer(c *ServerConfig) *Server {
	otelgrpc.UnaryServerInterceptor()
	if c == nil {
		if !flag.Parsed() {
			fmt.Fprint(os.Stderr, "[net/tcp] please call flag.Parse() before Init go tcp server, some configure may not effect.\n")
		}
		var err error
		if c, err = parseDSN(_tcpDSN); err != nil {
			panic(fmt.Sprintf("parseDSN error(%v)", err))
		}
	} else {
		fmt.Fprintf(os.Stderr, "[net/tcp] config will be deprecated, argument will be ignored.\n")
	}
	s := newServer()
	go tcpListen(c, s)
	s.lg.Info("starting tcp server on", zap.String("tcpAddr", c.Addr))
	return s
}

func newServer() *Server {
	return &Server{serviceMap: make(map[string]*service), lg: global.GetLogger().Named("tcp-server")}
}

// tcpListen start tcp listen.
func tcpListen(c *ServerConfig, s *Server) {
	l, err := net.Listen(c.Network, c.Addr)
	if err != nil {
		s.lg.Sugar().Errorf("net.Listen(tcpAddr:(%v)) error(%v)", c.Addr, err)
		panic(err)
	}
	// if process exit, then close the tcp bind
	defer func() {
		if err := l.Close(); err != nil {
			s.lg.Sugar().Errorf("listener.Close() error(%v)", err)
		}
	}()
	s.Accept(l)
}

func (s *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			s.lg.Error("tcp.Serve: accept:", zap.Error(err))
			return
		}
		go s.ServeConn(conn)
	}
}

func (s *Server) ServeConn(conn net.Conn) {
	srv := &serverCodec{
		rwc:  conn,
		addr: conn.RemoteAddr(),
	}
	s.serveCodec(srv)
}

func (s *Server) serveCodec(codec *serverCodec) {
	req := &codec.req
	for {
		// serve request
		service, argv, replyv, err := s.readRequest(codec)
		if err != nil {
			if err != io.EOF {
				s.lg.Error("read request error", zap.Error(err))
				break
			}
			if req.ctx == nil {
				break
			}
			errmsg := err.Error()
			s.sendResponse(req.ctx, codec, invalidRequest(&req.ProtoHeader), errmsg)
			break
		}
		go service.call(req.ctx, s, argv, replyv, codec)
	}
	codec.close()
}

func (s *Server) readRequest(codec *serverCodec) (service *service, argv, replyv *ProtoBuff, err error) {
	var req = &codec.req
	*req = Request{}
	if service, err = s.readRequestHeader(codec); err != nil {
		// keepreading
		if req.ctx == nil {
			return
		}
		return
	}
	if argv == nil {
		argv = &ProtoBuff{Header: &req.ProtoHeader}
	}
	var blockLen int
	if argv.Header != nil && argv.Header.BodyLength > 0 {
		if err = codec.readRequestBody(argv); err != nil {
			return
		}
		if blockLen, err = codec.readRequestBlockLen(); err != nil {
			return
		}
		if argv.Body != nil && blockLen > 0 {
			if err = codec.readRequestBlocks(argv, blockLen); err != nil {
				return
			}
		}
	}
	replyv = &ProtoBuff{Header: &req.ProtoHeader}
	return
}

func (s *Server) readRequestHeader(codec *serverCodec) (service *service, err error) {
	req := &codec.req
	var headerBuf = make([]byte, _headerLen)
	if _, err = codec.rwc.Read(headerBuf); err != nil {
		err = errors.Wrap(err, "read resp header error")
		return
	}
	req.Seq = binary.BigEndian.Uint64(headerBuf[0:_seq])
	req.ServiceMethod = getValidByteToString(headerBuf[_seq:_serviceMethod])
	req.BodyLength = binary.BigEndian.Uint32(headerBuf[_serviceMethod:_length])
	req.ctx = context.NewContext(ctx.Background(), req.ServiceMethod, codec.auth, req.Seq, req.BodyLength)
	service = s.serviceMap[req.ServiceMethod]
	if service == nil {
		err = errors.New("can't find service " + req.ServiceMethod + fmt.Sprintf("len(%d)", len(req.ServiceMethod)))
		return
	}
	return
}

var invalidRequest = func(h *ProtoHeader) *ProtoBuff {
	return &ProtoBuff{Header: h}
}

func (s *Server) sendResponse(c context.Context, codec *serverCodec, reply *ProtoBuff, errmsg string) {
	var (
		err  error
		ts   ProtoHeader
		resp = &codec.resp
	)
	if errmsg != "" {
		reply = &ProtoBuff{}
	}
	ts.ServiceMethod = c.ServiceMethod()
	ts.Seq = c.Seq()
	ts.Error = errmsg
	codec.sending.Lock()
	*resp = ts
	reply.Header = resp
	// Encode the response header
	if err = codec.writeResponse(reply); err != nil {
		s.lg.Error("writing response", zap.Error(err))
	}
	codec.sending.Unlock()
}

func (s *Server) RegisterName(name string, fn func(c ctx.Context, arg *ProtoBuff) (*ProtoBuff, error)) error {
	if err := s.register(fn, name); err != nil {
		return err
	}
	return s.RegisterName(_pingMethod, s.ping)
}

func (s *Server) register(fn func(c ctx.Context, arg *ProtoBuff) (*ProtoBuff, error), name string) error {
	if s.serviceMap == nil {
		s.serviceMap = make(map[string]*service)
	}
	ser := new(service)
	ser.method = name
	if _, present := s.serviceMap[ser.method]; present {
		return errors.New("tcp: service already defined: " + ser.method)
	}
	ser.fn = fn
	s.serviceMap[ser.method] = ser
	return nil
}

func (s *Server) Close() error {
	if s.lis != nil {
		return s.lis.Close()
	}
	return nil
}

func (s *Server) ping(c ctx.Context, arg *ProtoBuff) (*ProtoBuff, error) {
	return &ProtoBuff{}, nil
}

func (c *serverCodec) readRequestBlockLen() (int, error) {
	var bodyBuf = make([]byte, _bodyLen)
	length, err := io.ReadFull(c.rwc, bodyBuf)
	if err != nil {
		return 0, errors.Wrap(err, "read resp block len error")
	}
	if uint32(length) != _bodyLen {
		return 0, errors.New("bad resp block len data")
	}
	return int(binary.BigEndian.Uint32(bodyBuf[0:_bodyLen])), nil
}

func (c *serverCodec) readRequestBody(argv *ProtoBuff) error {
	var bodyBuf = make([]byte, argv.Header.BodyLength)
	length, err := io.ReadFull(c.rwc, bodyBuf)
	if err != nil {
		return errors.Wrap(err, "read resp body error")
	}
	if uint32(length) != argv.Header.BodyLength {
		return errors.New("bad resp body data")
	}
	decoder := json.NewDecoder(bytes.NewReader(bodyBuf))
	decoder.UseNumber()
	if err := decoder.Decode(&argv.Body); err != nil {
		return errors.Wrap(err, "read body json unmarshal error")
	}
	return nil
}

func (c *serverCodec) readRequestBlocks(argv *ProtoBuff, len int) error {
	var bodyBuf = make([]byte, len)
	length, err := io.ReadFull(c.rwc, bodyBuf)
	if err != nil {
		return errors.Wrap(err, "read resp body error")
	}
	if length != len {
		return errors.New("bad resp body data")
	}
	argv.Block = bodyBuf
	return nil
}

func (c *serverCodec) writeResponse(buff *ProtoBuff) error {
	// body
	var (
		bodyBs []byte
		err    error
	)
	if buff.Body != nil {
		bodyBs, err = json.Marshal(buff.Body)
		if err != nil {
			return errors.Wrap(err, "write req body error")
		}
		// body len
		buff.Header.BodyLength = uint32(len(bodyBs))
	}
	// write header
	if _, err := c.writeRequestHeader(buff.Header); err != nil {
		return err
	}
	if buff.Header.Error == "" && buff.Body != nil {
		// write body
		if _, err := c.rwc.Write(bodyBs); err != nil {
			return err
		}
		if err := c.writeRequestBlockLen(len(buff.Block)); err != nil {
			return err
		}
		if len(buff.Block) > 0 {
			// write block
			if _, err := c.rwc.Write(buff.Block); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *serverCodec) writeRequestHeader(header *ProtoHeader) (int, error) {
	var headerBuf = make([]byte, _headerLen)
	binary.BigEndian.PutUint64(headerBuf[0:_seq], header.Seq)
	copy(headerBuf[_seq:_serviceMethod], []byte(header.ServiceMethod))
	binary.BigEndian.PutUint32(headerBuf[_serviceMethod:_length], header.BodyLength)
	copy(headerBuf[_length:_error], []byte(header.Error))
	return c.rwc.Write(headerBuf)
}

func (c *serverCodec) writeRequestBlockLen(len int) error {
	var headerBuf = make([]byte, _bodyLen)
	binary.BigEndian.PutUint32(headerBuf[0:_bodyLen], uint32(len))
	_, err := c.rwc.Write(headerBuf)
	return err
}

func (s *service) call(c context.Context, server *Server, argv, replyv *ProtoBuff, codec *serverCodec) {
	var (
		err    error
		errmsg string
		reply  *ProtoBuff
	)
	defer func() {
		if err1 := recover(); err1 != nil {
			err = err1.(error)
			errmsg = err.Error()
			server.lg.Sugar().Errorf("tcp call panic: %v \n", err1)
			server.sendResponse(c, codec, replyv, errmsg)
		}
	}()
	if reply, err = s.fn(c.Ctx(), argv); err != nil {
		server.sendResponse(c, codec, replyv, err.Error())
		return
	}
	if reply != nil {
		replyv.Body = reply.Body
		replyv.Block = reply.Block
	}
	server.sendResponse(c, codec, replyv, errmsg)
}

func (c *serverCodec) close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.rwc.Close()
}
