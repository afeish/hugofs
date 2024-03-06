package tcp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	xtime "github.com/afeish/hugo/pkg/tcp/time"
	"github.com/afeish/hugo/pkg/tracing"
	"github.com/afeish/hugo/pkg/util/mem"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/pingcap/log"
	"github.com/pkg/errors"
)

const (
	_pingDuration = time.Second * 1
)

var (
	ErrShutdown = errors.New("connection is shut down")
	ErrNoClient = errors.New("no tcp client")
	ErrDeadline = errors.New("tcp deadline")
	errClient   = new(client)
)

type Client struct {
	addr    string
	timeout xtime.Duration
	quit    chan struct{}
	client  atomic.Value
}

type client struct {
	codec    *clientCodec
	reqMutex sync.Mutex
	request  Request

	timeout    time.Duration
	mutex      sync.Mutex
	pending    map[uint64]*Call
	seq        uint64
	closing    bool
	shutdown   bool
	remoteAddr string
}

type Call struct {
	ServiceMethod string
	Args          *ProtoBuff
	Reply         *ProtoBuff
	Timeout       time.Duration
	Error         error
	Done          chan *Call
	ctx           context.Context
}

type clientCodec struct {
	rwc io.ReadWriteCloser
	ctx context.Context
}

func Dial(ctx context.Context, addr string, timeout xtime.Duration) *Client {
	client := &Client{
		addr:    addr,
		timeout: timeout,
		quit:    make(chan struct{}),
	}
	// timeout
	if timeout <= 0 {
		client.timeout = xtime.Duration(30 * time.Second)
	}
	client.timeout = timeout
	rc, err := dial(ctx, "tcp", addr, time.Duration(timeout))
	if err != nil {
		log.S().Error("dial(%s, %s) error(%v)", "tcp", addr, err)
	} else {
		client.client.Store(rc)
	}
	log.Debug("start connect to storage tcp", zap.String("addr", addr))
	go client.ping()
	return client
}

func dial(ctx context.Context, network, addr string, timeout time.Duration) (*client, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		err = errors.WithStack(err)
		return nil, err
	}
	return newClient(ctx, timeout, conn)
}

func newClient(ctx context.Context, timeout time.Duration, conn net.Conn) (*client, error) {
	client := &clientCodec{conn, ctx}
	c := newClientWithCodec(client)
	c.timeout = timeout
	c.remoteAddr = conn.RemoteAddr().String()
	return c, nil
}

func (call *Call) done() {
	select {
	case call.Done <- call:
		// ok
	default:
		// We don't want to block here. It is the caller's responsibility to make
		// sure the channel has enough buffer space. See comment in Go().
		log.Warn("tcp: discarding Call reply due to insufficient Done chan capacity")
	}
}

func (client *client) send(call *Call) {
	_, span := tracing.Tracer.Start(call.ctx, "client send")
	defer span.End()
	client.reqMutex.Lock()
	defer client.reqMutex.Unlock()
	// Register this call.
	client.mutex.Lock()
	if client.shutdown || client.closing {
		call.Error = ErrShutdown
		client.mutex.Unlock()
		call.done()
		return
	}
	seq := client.seq
	client.seq++
	client.pending[seq] = call
	client.mutex.Unlock()

	// Encode and send the request.
	client.request.Seq = seq
	client.request.ServiceMethod = call.ServiceMethod
	client.request.BodyLength = 0
	client.request.Error = ""
	client.request.timeout = call.Timeout
	if call.Args == nil {
		call.Args = new(ProtoBuff)
	}
	err := client.codec.WriteRequest(&client.request, call.Args)
	if err != nil {
		err = errors.WithStack(err)
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply *ProtoBuff) (err error) {
	ctx, span := tracing.Tracer.Start(ctx, fmt.Sprintf("tcp client call serviceMethod: %s", serviceMethod))
	defer span.End()
	var (
		ok      bool
		rc      *client
		call    *Call
		cancel  func()
		timeout = time.Duration(c.timeout)
	)
	if rc, ok = c.client.Load().(*client); !ok || rc == errClient {
		log.S().Errorf("client is errClient (no tcp client) by ping addr(%s) error", c.addr)
		return ErrNoClient
	}
	deliver := true
	if deadline, ok := ctx.Deadline(); ok {
		if ctimeout := time.Until(deadline); ctimeout < timeout {
			timeout = ctimeout
			deliver = false
		}
	}
	if deliver {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	call = &Call{
		Args:          args,
		Reply:         reply,
		Timeout:       timeout,
		ServiceMethod: serviceMethod,
		ctx:           ctx,
	}
	if serviceMethod != "" {
		call.Args.Header.ServiceMethod = serviceMethod
	}
	rc.Do(call)
	select {
	case call = <-call.Done:
		err = call.Error
	case <-ctx.Done():
		b, _ := json.Marshal(call.Args.Body)
		log.S().Errorf("tcp timout seq(%d) body(%s) ", call.Args.Header.Seq, b)
		err = ErrDeadline
	}
	return
}

func (c *client) Do(call *Call) {
	c.do(call, make(chan *Call, 1))
}

func (c *clientCodec) WriteRequest(r *Request, buff *ProtoBuff) error {
	buff.Header = &r.ProtoHeader
	var (
		bodyBs []byte
		err    error
	)
	if buff.Body != nil {
		// body
		bodyBs, err = json.Marshal(buff.Body)
		if err != nil {
			return errors.Wrap(err, "write req body error")
		}
		// body len
		buff.Header.BodyLength = uint32(len(bodyBs))
	}
	// write header
	if err := c.writeRequestHeader(buff.Header); err != nil {
		return err
	}
	if buff.Body != nil {
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

func (c *clientCodec) writeRequestHeader(header *ProtoHeader) error {
	var headerBuf = make([]byte, _headerLen)
	binary.BigEndian.PutUint64(headerBuf[0:_seq], header.Seq)
	copy(headerBuf[_seq:_serviceMethod], []byte(header.ServiceMethod))
	binary.BigEndian.PutUint32(headerBuf[_serviceMethod:_length], header.BodyLength)
	_, err := c.rwc.Write(headerBuf)
	return err
}

func (c *clientCodec) writeRequestBlockLen(len int) error {
	var headerBuf = make([]byte, _bodyLen)
	binary.BigEndian.PutUint32(headerBuf[0:_bodyLen], uint32(len))
	_, err := c.rwc.Write(headerBuf)
	return err
}

func (c *clientCodec) ReadResponseHeader(header *ProtoHeader) error {
	var headerBuf = make([]byte, _headerLen)
	if _, err := c.rwc.Read(headerBuf); err != nil {
		return errors.Wrap(err, "read resp header error")
	}
	header.Seq = binary.BigEndian.Uint64(headerBuf[0:_seq])
	header.ServiceMethod = getValidByteToString(headerBuf[_seq:_serviceMethod])
	header.BodyLength = binary.BigEndian.Uint32(headerBuf[_serviceMethod:_length])
	header.Error = getValidByteToString(headerBuf[_length:_error])
	// Tracer
	_, span := tracing.Tracer.Start(c.ctx, "tcp client read header", trace.WithAttributes(attribute.String("service_method", header.ServiceMethod), attribute.String("error", header.Error)))
	span.End()
	return nil
}

func (c *clientCodec) ReadResponseBody(body *ProtoBuff, len int) error {
	var bodyBuf = make([]byte, len)
	length, err := io.ReadFull(c.rwc, bodyBuf)
	if err != nil {
		return errors.Wrap(err, "read resp body error")
	}
	if length != len {
		return errors.New("bad resp body data")
	}
	decoder := json.NewDecoder(bytes.NewReader(bodyBuf))
	decoder.UseNumber()
	decoder.Decode(&body.Body)
	return nil
}

func (c *clientCodec) ReadResponseBlockLen() (int, error) {
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

func (c *clientCodec) ReadResponseBlocks(block *ProtoBuff, len int) error {
	_, span := tracing.Tracer.Start(c.ctx, "tcp client read block")
	defer span.End()
	var bodyBuf = mem.Allocate(len)
	length, err := io.ReadFull(c.rwc, bodyBuf)
	if err != nil {
		return errors.Wrap(err, "read resp body error")
	}
	if length != len {
		return errors.New("bad resp body data")
	}
	block.Block = bodyBuf
	return nil
}

func (client *client) do(call *Call, done chan *Call) {
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel. If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("tcp: done channel is unbuffered")
		}
	}
	call.Done = done
	client.send(call)
}

func (client *client) Go(serviceMethod string, args, reply *ProtoBuff, done chan *Call) *Call {
	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply
	call.ctx = context.Background()
	client.do(call, done)
	return call
}

func (client *client) Call(serviceMethod string, args, reply *ProtoBuff) (err error) {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}

func newClientWithCodec(codec *clientCodec) *client {
	client := &client{
		codec:   codec,
		pending: make(map[uint64]*Call),
	}
	go client.input()
	return client
}

func (c *client) input() {
	var err error
	var response ProtoHeader
	for err == nil {
		response = ProtoHeader{}
		if err = c.codec.ReadResponseHeader(&response); err != nil {
			log.S().Errorf("tcp read header error(%v)", err)
			break
		}
		c.mutex.Lock()
		seq := response.Seq
		call, ok := c.pending[seq]
		if !ok {
			log.S().Errorf("tcp client seq(%d) call is null", seq)
		}
		delete(c.pending, seq)
		c.mutex.Unlock()
		switch {
		case call == nil:
			if err = c.readBody(&ProtoBuff{}, int(response.BodyLength)); err != nil {
				err = errors.New("reading error body: " + err.Error())
			}
		case response.Error != "":
			call.Error = ServerError(response.Error)
			call.done()
		default:
			call.Reply.Header = &response
			if err = c.readBody(call.Reply, int(call.Reply.Header.BodyLength)); err != nil {
				call.Error = err
			}
			call.done()
		}
	}
	c.reqMutex.Lock()
	c.mutex.Lock()
	c.shutdown = true
	closing := c.closing
	if err == io.EOF {
		if closing {
			err = ErrShutdown
		} else {
			err = io.ErrUnexpectedEOF
		}
	}
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
	c.mutex.Unlock()
	c.reqMutex.Unlock()
	if err != io.EOF && !closing {
		log.Error("tcp: client protocol error:", zap.Error(err))
	}
}

func (client *client) readBody(buff *ProtoBuff, bodylen int) error {
	if bodylen == 0 {
		return nil
	}
	if err := client.codec.ReadResponseBody(buff, bodylen); err != nil {
		return errors.New("reading body " + err.Error())
	}
	blocklen, err := client.codec.ReadResponseBlockLen()
	if err != nil {
		return errors.New("reading body " + err.Error())
	}
	if buff.Body != nil && blocklen > 0 {
		if err := client.codec.ReadResponseBlocks(buff, blocklen); err != nil {
			return errors.New("reading block " + err.Error())
		}
	}
	return nil
}

// Close close the tcp client.
func (client *client) Close() error {
	defer client.mutex.Unlock()
	client.mutex.Lock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.codec.Close()
}

func (c *clientCodec) Close() error {
	return errors.WithStack(c.rwc.Close())
}

type ServerError string

func (e ServerError) Error() string {
	return string(e)
}

func (c *Client) ping() {
	var (
		err       error
		cancel    func()
		call      *Call
		ctx       context.Context
		client, _ = c.client.Load().(*client)
	)
	for {
		select {
		case <-c.quit:
			c.client.Store(errClient)
			if client != nil {
				client.Close()
			}
			return
		default:
		}
		if client == nil || err != nil {
			if client, err = dial(ctx, "tcp", c.addr, time.Duration(c.timeout)); err != nil {
				log.S().Errorf("dial(%s, %s) error(%v)", "tcp", c.addr, err)
				time.Sleep(_pingDuration)
				continue
			}
			c.client.Store(client)
		}
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(c.timeout))
		select {
		case call = <-client.Go(_pingMethod, &ProtoBuff{}, &ProtoBuff{}, make(chan *Call, 1)).Done:
			err = call.Error
		case <-ctx.Done():
			err = ErrDeadline
		}
		cancel()
		if err != nil {
			switch err {
			case ErrShutdown, io.ErrUnexpectedEOF, ErrDeadline:
				log.S().Errorf("tcp ping error beiTle addr(%s) error(%v)", c.addr, err)
				c.client.Store(errClient)
				client.Close()
			default:
				err = nil
			}
		}
		time.Sleep(_pingDuration)
	}
}

func (c *Client) Close() {
	close(c.quit)
}
